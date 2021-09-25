package scrapers

import (
	"net"
	"regexp"
	"strings"

	"dev.hon.one/vanadium/common"
	log "github.com/sirupsen/logrus"
)

var vyosVendorRegex = regexp.MustCompile(`^Hardware vendor: +([^ ]|[^ ].*[^ ]+) *$`)
var vyosModelRegex = regexp.MustCompile(`^Hardware model: +([^ ]|[^ ].*[^ ]+) *$`)
var vyosVersionRegex = regexp.MustCompile(`^Version: +([^ ]|[^ ].*[^ ]+) *$`)
var vyosInterfaceRegex = regexp.MustCompile(`^([^ ]+) +UP +(.+)$`) // Only up interfaces with addresses
var vyosNeighborRegex = regexp.MustCompile(`^([^ ]+).* ([^ ]+)$`)
var vyosNeighborDevRegex = regexp.MustCompile(`dev ([^ ]+)`)
var vyosNeighborLladdrRegex = regexp.MustCompile(`lladdr ([^ ]+)`)

// VyOSSSH - Scrape a VyOS router using SSH.
func VyOSSSH(device common.Device) bool {
	if !vyOSSSHVersion(device) {
		return false
	}
	if !vyOSSSHInterfaces(device) {
		return false
	}
	if !vyOSSSHNeighbors(device) {
		return false
	}
	return true
}

func vyOSSSHVersion(device common.Device) bool {
	lines, ok := runSSHCommand(device, "/usr/libexec/vyos/op_mode/show_version.py")
	if !ok {
		return false
	}
	vendor := ""
	model := ""
	version := ""
	for _, line := range lines {
		vendorResult := vyosVendorRegex.FindStringSubmatch(line)
		modelResult := vyosModelRegex.FindStringSubmatch(line)
		versionResult := vyosVersionRegex.FindStringSubmatch(line)
		if vendorResult != nil {
			vendor = vendorResult[1]
		}
		if modelResult != nil {
			model = modelResult[1]
		}
		if versionResult != nil {
			version = versionResult[1]
		}
	}
	// TODO store
	log.WithFields(log.Fields{
		"device":  device.Address,
		"vendor":  vendor,
		"model":   model,
		"version": version,
	}).Trace("Found device info")
	return true
}

func vyOSSSHInterfaces(device common.Device) bool {
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
		interfaceNetworks := make(map[string][]string)
		for _, rawAddress := range strings.Fields(strings.TrimSpace(rawIPAddresses)) {
			ipAddress, ipNetwork, err := net.ParseCIDR(rawAddress)
			if !checkDeviceFailure(device, "Malformed IP CIDR address", err) {
				continue
			}

			// Skip localhost and link-local
			if ipAddress.IsLoopback() || ipAddress.IsLinkLocalUnicast() {
				continue
			}

			addressList := interfaceNetworks[l3Interface]
			addressList = append(addressList, ipNetwork.String())
			log.WithFields(log.Fields{
				"device":       device.Address,
				"l3_interface": l3Interface,
				"network":      ipNetwork,
			}).Trace("Found L3 interface network")
		}
	}

	// TODO store result

	return true
}

func vyOSSSHNeighbors(device common.Device) bool {
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
			showDeviceFailure(device, "Malformed IP address")
			continue
		}
		macAddress, err := net.ParseMAC(rawMACAddress)
		if !checkDeviceFailure(device, "Malformed MAC address", err) {
			continue
		}

		// Skip link-local
		if ipAddress.IsLinkLocalUnicast() {
			continue
		}

		// TODO store
		log.WithFields(log.Fields{
			"device":       device.Address,
			"l3_interface": l3Interface,
			"ip_address":   ipAddress,
			"mac_address":  macAddress,
		}).Trace("Found neighbor entry")
	}
	return true
}
