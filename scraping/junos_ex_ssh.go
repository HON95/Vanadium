package scraping

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/db"
)

var junosOperPromptRegex = regexp.MustCompile(`[^@]+@[^>]+>`)
var junosVersionBeginRegex = regexp.MustCompile(`show version`)
var junosModelRegex = regexp.MustCompile(`Model: ([^ ]+)`)
var junosSoftwareVersionRegex = regexp.MustCompile(`Junos: ([^ ]+)`)
var junosVLANTableBeginRegex = regexp.MustCompile(`show vlans brief`)
var junosVLANTableEntryRegex = regexp.MustCompile(`^ *([^ ]+) +([0-9]+) `)
var junosMACTableBeginRegex = regexp.MustCompile(`show ethernet-switching table`)
var junosMACTableEntryRegex = regexp.MustCompile(`^ *([^ ]+) +([^ ]+) +([^ ]+) +([^ ]+) +([^ ]+) *$`)
var junosInterfaceNetworksBeginRegex = regexp.MustCompile(`show interfaces terse`)
var junosInterfaceNetworksEntryRegex = regexp.MustCompile(`^([^ ]+) +([^ ]+) +([^ ]+)(?: +([^ ]+) +([^ ]+))?`)
var junosInterfaceNetworksExtraEntryRegex = regexp.MustCompile(`^ +([^ ]+)? +([^ ]+)`)
var junosARPTableBeginRegex = regexp.MustCompile(`show arp no-resolve`)
var junosARPTableEntryRegex = regexp.MustCompile(`^([^ ]+) +([^ ]+) +([^ ]+) +([^ ]+) *$`)
var junosNDPTableBeginRegex = regexp.MustCompile(`show ipv6 neighbors`)
var junosNDPTableEntryRegex = regexp.MustCompile(`^([^ ]+) +([^ ]+) +([^ ]+) +([^ ]+) +([^ ]+) +([^ ]+) +([^ ]+) *$`)

// JunosEXSSH - Scrape a Juniper EX switch running Junos using SSH.
func JunosEXSSH(device common.Device, startTime time.Time) bool {
	_, sshClient, sshClientOpenSuccess := openSSHClient(device)
	if !sshClientOpenSuccess {
		return false
	}
	defer sshClient.Close()

	// Setup session
	session, err := sshClient.NewSession()
	if !checkDeviceWeakFailure(device, "Failed to start session", err) {
		return false
	}
	stdinWriter, err := session.StdinPipe()
	if !checkDeviceWeakFailure(device, "Failed to get STDIN pipe", err) {
		return false
	}
	stdoutReader, err := session.StdoutPipe()
	if !checkDeviceWeakFailure(device, "Failed to get STDOUT pipe", err) {
		return false
	}
	stderrReader, err := session.StderrPipe()
	if !checkDeviceWeakFailure(device, "Failed to get STDERR pipe", err) {
		return false
	}
	err = session.Shell()
	if !checkDeviceWeakFailure(device, "Failed to start shell", err) {
		return false
	}

	// Process sections in sequence, line-by-line
	lineChannel := followSSHStreamLines(device, "STDOUT", stdoutReader)
	drainSSHStreamLines(device, "STDERR", stderrReader, false)
	var currentOutput outputReaderResult

	// Get device info (allow missing info)
	stdinWriter.Write([]byte("show version\n\n"))
	stdinWriter.Write([]byte("help\n\n")) // Push output
	model := ""
	softwareVersion := ""
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		result := junosVersionBeginRegex.FindStringSubmatch(currentOutput.Line)
		if result != nil {
			break
		}
	}
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		endResult := junosOperPromptRegex.FindStringSubmatch(currentOutput.Line)
		if endResult != nil {
			// Reached prompt after command output, all output should have been captured now
			break
		}
		modelResult := junosModelRegex.FindStringSubmatch(currentOutput.Line)
		softwareVersionResult := junosSoftwareVersionRegex.FindStringSubmatch(currentOutput.Line)
		if modelResult != nil {
			model = modelResult[1]
		}
		if softwareVersionResult != nil {
			softwareVersion = softwareVersionResult[1]
		}
	}
	vendor := "Juniper"
	sourceDeviceEntry := common.SourceDeviceEntry{
		Time:     startTime,
		Source:   device.Address,
		Vendor:   vendor,
		Model:    model,
		Software: fmt.Sprintf("Junos %v", softwareVersion),
		Other:    "",
	}
	db.StoreSourceDeviceEntry(sourceDeviceEntry)

	// Get VLANs
	stdinWriter.Write([]byte("show vlans brief\n\n"))
	stdinWriter.Write([]byte("help\n\n")) // Push output
	vlans := make(map[string]uint)
	// Default isn't automatically added since it isn't defined with a tag/VID
	vlans["default"] = 0
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		result := junosVLANTableBeginRegex.FindStringSubmatch(currentOutput.Line)
		if result != nil {
			break
		}
	}
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		endResult := junosOperPromptRegex.FindStringSubmatch(currentOutput.Line)
		if endResult != nil {
			// Reached prompt after command output, all output should have been captured now
			break
		}
		result := junosVLANTableEntryRegex.FindStringSubmatch(currentOutput.Line)
		if result == nil {
			// Skip line if no match
			continue
		}
		vlanName := result[1]
		vlanIDStr := result[2]
		vlanID, err := strconv.ParseUint(vlanIDStr, 10, 12)
		if !checkDeviceWeakFailure(device, "Failed to parse VLAN ID", err) {
			continue
		}
		if vlanID < 0 || vlanID >= 4096 {
			showDeviceWeakFailure(device, "VLAN ID out of range")
			continue
		}

		vlans[vlanName] = uint(vlanID)
	}

	// Get MAC table
	stdinWriter.Write([]byte("show ethernet-switching table\n\n"))
	stdinWriter.Write([]byte("help\n\n")) // Push output
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		result := junosMACTableBeginRegex.FindStringSubmatch(currentOutput.Line)
		if result != nil {
			break
		}
	}
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		endResult := junosOperPromptRegex.FindStringSubmatch(currentOutput.Line)
		if endResult != nil {
			// Reached prompt after command output, all output should have been captured now
			break
		}
		result := junosMACTableEntryRegex.FindStringSubmatch(currentOutput.Line)
		if result == nil {
			// Skip line if no match
			continue
		}
		vlanName := result[1]
		rawMACAddress := result[2]
		l2Interface := result[5]
		// Ignore "unknown MAC address" entries
		if rawMACAddress == "*" {
			continue
		}
		isSelf := l2Interface == "Router"
		vlanID, ok := vlans[vlanName]
		if !ok {
			showDeviceWeakFailure(device, "VLAN ID not found")
			continue
		}
		macAddress, err := net.ParseMAC(rawMACAddress)
		if !checkDeviceWeakFailure(device, "Malformed MAC address", err) {
			continue
		}

		l2DeviceEntry := common.L2DeviceEntry{
			Time:        startTime,
			Source:      device.Address,
			MACAddress:  macAddress.String(),
			VLANID:      vlanID,
			VLANName:    vlanName,
			L2Interface: l2Interface,
			IsSelf:      isSelf,
		}
		db.StoreL2DeviceEntry(l2DeviceEntry)
	}

	// Get connected networks
	stdinWriter.Write([]byte("show interfaces terse\n\n"))
	stdinWriter.Write([]byte("help\n\n")) // Push output
	var lastL3InterfaceID string
	var lastL3InterfaceProto string
	interfaceNetworks := make(map[string][]net.IPNet)
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		result := junosInterfaceNetworksBeginRegex.FindStringSubmatch(currentOutput.Line)
		if result != nil {
			break
		}
	}
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		endResult := junosOperPromptRegex.FindStringSubmatch(currentOutput.Line)
		if endResult != nil {
			// Reached prompt after command output, all output should have been captured now
			break
		}
		baseResult := junosInterfaceNetworksEntryRegex.FindStringSubmatch(currentOutput.Line)
		extraResult := junosInterfaceNetworksExtraEntryRegex.FindStringSubmatch(currentOutput.Line)
		var l3Interface string
		var proto string
		var rawIPNetwork string
		if baseResult != nil {
			l3Interface = baseResult[1]
			proto = baseResult[4]
			rawIPNetwork = baseResult[5]
			lastL3InterfaceID = l3Interface
			lastL3InterfaceProto = proto
		} else if extraResult != nil {
			l3Interface = lastL3InterfaceID
			proto = lastL3InterfaceProto
			if extraResult[1] != "" {
				proto = extraResult[1]
			}
			rawIPNetwork = extraResult[2]
		} else {
			continue
		}

		// Skip missing fields
		if l3Interface == "" || proto == "" || rawIPNetwork == "" {
			continue
		}
		// Skip table header and weird interfaces
		if l3Interface == "Interface" || strings.HasPrefix(l3Interface, "bme0") || strings.HasPrefix(l3Interface, "jsrv") || strings.HasPrefix(l3Interface, "lo0") {
			continue
		}
		// Skip weird protos
		if proto != "inet" && proto != "inet6" {
			continue
		}

		// Parse address to network
		ipAddress, ipNetwork, err := net.ParseCIDR(rawIPNetwork)
		if !checkDeviceWeakFailure(device, "Malformed IP CIDR address", err) {
			continue
		}

		// Skip localhost and link-local
		if ipAddress.IsLoopback() || ipAddress.IsLinkLocalUnicast() {
			continue
		}

		interfaceNetworks[l3Interface] = append(interfaceNetworks[l3Interface], *ipNetwork)
	}

	// Get IPv4 ARP table
	stdinWriter.Write([]byte("show arp no-resolve\n\n"))
	stdinWriter.Write([]byte("help\n\n")) // Push output
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		result := junosARPTableBeginRegex.FindStringSubmatch(currentOutput.Line)
		if result != nil {
			break
		}
	}
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		endResult := junosOperPromptRegex.FindStringSubmatch(currentOutput.Line)
		if endResult != nil {
			// Reached prompt after command output, all output should have been captured now
			break
		}
		result := junosARPTableEntryRegex.FindStringSubmatch(currentOutput.Line)
		if result == nil {
			continue
		}
		rawMACAddress := result[1]
		rawIPAddress := result[2]
		l3Interface := result[3]

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
		var ipNetwork *net.IPNet = nil
		for _, network := range interfaceNetworks[l3Interface] {
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

	// Get IPv6 NDP table
	stdinWriter.Write([]byte("show ipv6 neighbors\n\n"))
	stdinWriter.Write([]byte("help\n\n")) // Push output
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		result := junosNDPTableBeginRegex.FindStringSubmatch(currentOutput.Line)
		if result != nil {
			break
		}
	}
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status != outputReaderOK {
			return false
		}
		endResult := junosOperPromptRegex.FindStringSubmatch(currentOutput.Line)
		if endResult != nil {
			// Reached prompt after command output, all output should have been captured now
			break
		}
		result := junosNDPTableEntryRegex.FindStringSubmatch(currentOutput.Line)
		if result == nil {
			continue
		}
		rawIPAddress := result[1]
		rawMACAddress := result[2]
		l3Interface := result[7]

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
		var ipNetwork *net.IPNet = nil
		for _, network := range interfaceNetworks[l3Interface] {
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

	// Exit
	stdinWriter.Write([]byte("exit\n"))
	for {
		currentOutput = <-lineChannel
		if currentOutput.Status == outputReaderDone {
			return true
		}
		if currentOutput.Status == outputReaderError {
			return false
		}
	}
}
