package scraping

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/db"
	log "github.com/sirupsen/logrus"
)

var vyosSoftwareVersionRegex = regexp.MustCompile(`^Version: +([^ ]|[^ ].*[^ ]+) *$`)
var vyosHardwareVendorRegex = regexp.MustCompile(`^Hardware vendor: +([^ ]|[^ ].*[^ ]+) *$`)
var vyosHardwareModelRegex = regexp.MustCompile(`^Hardware model: +([^ ]|[^ ].*[^ ]+) *$`)
var vyosInterfaceRegex = regexp.MustCompile(`^([^ ]+) +UP +(.+)$`) // Only up interfaces with addresses
var vyosNeighborRegex = regexp.MustCompile(`^([^ ]+).* ([^ ]+)$`)
var vyosNeighborDevRegex = regexp.MustCompile(`dev ([^ ]+)`)
var vyosNeighborLladdrRegex = regexp.MustCompile(`lladdr ([^ ]+)`)

// VyOSSSH - Scrape a VyOS router using SSH.
func VyOSSSH(device common.Device, startTime time.Time) bool {
	if !vyOSSSHVersion(device, startTime) {
		return false
	}
	interfaceNetworks := make(map[string][]net.IPNet)
	if !vyOSSSHInterfaces(device, startTime, interfaceNetworks) {
		return false
	}
	if !vyOSSSHNeighbors(device, startTime, interfaceNetworks) {
		return false
	}
	return true
}

func vyOSSSHVersion(device common.Device, startTime time.Time) bool {
	lines, ok := runSSHCommand(device, "/usr/libexec/vyos/op_mode/show_version.py")
	if !ok {
		return false
	}
	vendor := "VyOS"
	softwareVersion := ""
	hardwareVendor := ""
	hardwareModel := ""
	for _, line := range lines {
		softwareVersionResult := vyosSoftwareVersionRegex.FindStringSubmatch(line)
		hardwareVendorResult := vyosHardwareVendorRegex.FindStringSubmatch(line)
		hardwareModelResult := vyosHardwareModelRegex.FindStringSubmatch(line)
		if softwareVersionResult != nil {
			softwareVersion = softwareVersionResult[1]
		}
		if hardwareVendorResult != nil {
			hardwareVendor = hardwareVendorResult[1]
		}
		if hardwareModelResult != nil {
			hardwareModel = hardwareModelResult[1]
		}
	}

	sourceDeviceEntry := common.SourceDeviceEntry{
		Time:     startTime,
		Source:   device.Address,
		Vendor:   vendor,
		Model:    "",
		Software: fmt.Sprintf("VyOS %v", softwareVersion),
		Other:    fmt.Sprintf("%v, %v", hardwareVendor, hardwareModel),
	}
	db.StoreSourceDeviceEntry(sourceDeviceEntry)

	return true
}

func vyOSSSHInterfaces(device common.Device, startTime time.Time, interfaceNetworks map[string][]net.IPNet) bool {
	lines, ok := runSSHCommand(device, "ip --brief address")
	if !ok {
		return false
	}

	for _, line := range lines {
		result := vyosInterfaceRegex.FindStringSubmatch(line)
		if result == nil {
			continue
		}
		l3Interface := result[1]
		rawIPAddresses := result[2]

		// Parse IP networks
		for _, rawAddress := range strings.Fields(strings.TrimSpace(rawIPAddresses)) {
			ipAddress, ipNetwork, err := net.ParseCIDR(rawAddress)
			if !checkDeviceWeakFailure(device, "Malformed IP CIDR address", err) {
				continue
			}

			// Skip localhost and link-local
			if ipAddress.IsLoopback() || ipAddress.IsLinkLocalUnicast() {
				continue
			}

			interfaceNetworks[l3Interface] = append(interfaceNetworks[l3Interface], *ipNetwork)
		}
	}

	return true
}

func vyOSSSHNeighbors(device common.Device, startTime time.Time, interfaceNetworks map[string][]net.IPNet) bool {
	lines, ok := runSSHCommand(device, "ip -4 neighbor ; ip -6 neighbor")
	if !ok {
		return false
	}
	for _, line := range lines {
		if line == "" {
			continue
		}
		mainResult := vyosNeighborRegex.FindStringSubmatch(line)
		if mainResult == nil {
			log.WithFields(log.Fields{
				"device": device.Address,
			}).Tracef("Failed to parse neighbor line: %v", line)
			continue
		}
		rawIPAddress := mainResult[1]
		neighborState := mainResult[2]
		devResult := vyosNeighborDevRegex.FindStringSubmatch(line)
		l3Interface := ""
		if devResult != nil {
			l3Interface = devResult[1]
		}
		lladdrResult := vyosNeighborLladdrRegex.FindStringSubmatch(line)
		rawMACAddress := ""
		if lladdrResult != nil {
			rawMACAddress = lladdrResult[1]
		}

		// Ignore bad states (see ip-neighbour(8))
		switch neighborState {
		case "PERMANENT":
		case "NOARP":
		case "REACHABLE":
		default:
			continue
		}
		// Require L3 interface and MAC address
		if l3Interface == "" || rawMACAddress == "" {
			log.WithFields(log.Fields{
				"device": device.Address,
			}).Warnf("Port or MAC address not found for neighbor with OK state: %v", line)
			continue
		}

		// Parse addresses
		ipAddress := net.ParseIP(rawIPAddress)
		if ipAddress == nil {
			showDeviceWeakFailure(device, "Malformed IP address")
			continue
		}
		macAddress, err := net.ParseMAC(rawMACAddress)
		if !checkDeviceWeakFailure(device, "Malformed MAC address", err) {
			continue
		}

		// Skip link-local
		if ipAddress.IsLinkLocalUnicast() {
			continue
		}

		// Find connected IP network
		networks, ok := interfaceNetworks[l3Interface]
		if !ok {
			showDeviceWeakFailure(device, fmt.Sprintf("Connected network interface not found: %v", l3Interface))
			continue
		}
		var ipNetwork *net.IPNet = nil
		for _, network := range networks {
			if network.Contains(ipAddress) {
				ipNetwork = &network
				break
			}
		}
		if ipNetwork == nil {
			showDeviceWeakFailure(device, "Connected network not found")
			continue
		}

		l3DeviceEntry := common.L3DeviceEntry{
			Time:        startTime,
			Source:      device.Address,
			IPAddress:   ipAddress.String(),
			MACAddress:  macAddress.String(),
			L3Interface: l3Interface,
			IPNetwork:   ipNetwork.String(),
		}
		db.StoreL3DeviceEntry(l3DeviceEntry)
	}
	return true
}
