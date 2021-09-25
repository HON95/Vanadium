package scrapers

import (
	"net"
	"regexp"
	"strconv"
	"strings"

	"dev.hon.one/vanadium/common"
	log "github.com/sirupsen/logrus"
)

var junosOperPromptRegex = regexp.MustCompile(`[^@]+@[^>]+>`)
var junosVersionBeginRegex = regexp.MustCompile(`show version`)
var junosModelRegex = regexp.MustCompile(`Model: ([^ ]+)`)
var junosVersionRegex = regexp.MustCompile(`Junos: ([^ ]+)`)
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
func JunosEXSSH(device common.Device) bool {
	_, sshClient, sshClientOpenSuccess := openSSHClient(device)
	if !sshClientOpenSuccess {
		return false
	}
	defer sshClient.Close()

	// Setup session
	session, err := sshClient.NewSession()
	if !checkDeviceFailure(device, "Failed to start session", err) {
		return false
	}
	stdinWriter, err := session.StdinPipe()
	if !checkDeviceFailure(device, "Failed to get STDIN pipe", err) {
		return false
	}
	stdoutReader, err := session.StdoutPipe()
	if !checkDeviceFailure(device, "Failed to get STDOUT pipe", err) {
		return false
	}
	stderrReader, err := session.StderrPipe()
	if !checkDeviceFailure(device, "Failed to get STDERR pipe", err) {
		return false
	}
	err = session.Shell()
	if !checkDeviceFailure(device, "Failed to start shell", err) {
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
	version := ""
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
		versionResult := junosVersionRegex.FindStringSubmatch(currentOutput.Line)
		if modelResult != nil {
			model = modelResult[1]
		}
		if versionResult != nil {
			version = versionResult[1]
		}
	}
	log.WithFields(log.Fields{
		"device":  device.Address,
		"model":   model,
		"version": version,
	}).Trace("Found device info")

	// Get VLANs
	stdinWriter.Write([]byte("show vlans brief\n\n"))
	stdinWriter.Write([]byte("help\n\n")) // Push output
	vlans := make(map[string]int)
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
		vlanID, err := strconv.Atoi(vlanIDStr)
		if !checkDeviceFailure(device, "Failed to parse VLAN ID", err) {
			continue
		}
		if vlanID < 0 || vlanID >= 4096 {
			showDeviceFailure(device, "VLAN ID out of range")
			continue
		}
		vlans[vlanName] = vlanID
		log.WithFields(log.Fields{
			"device":    device.Address,
			"vlan_name": vlanName,
			"vlan_id":   vlanID,
		}).Trace("Found VLAN")
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
		learnType := result[3]
		l2Interface := result[5]
		// Ignore header, self-addresses, static addresses, etc.
		if learnType != "Learn" && learnType != "Static" {
			continue
		}
		vlanID, ok := vlans[vlanName]
		if !ok {
			showDeviceFailure(device, "VLAN ID not found")
			continue
		}
		macAddress, err := net.ParseMAC(rawMACAddress)
		if !checkDeviceFailure(device, "Malformed MAC address", err) {
			continue
		}
		log.WithFields(log.Fields{
			"device":       device.Address,
			"mac_addr":     macAddress,
			"vlan_id":      vlanID,
			"vlan_name":    vlanName,
			"l2_interface": l2Interface,
		}).Trace("Found MAC entry")
		// TODO store it
	}

	// Get connected networks
	stdinWriter.Write([]byte("show interfaces terse\n\n"))
	stdinWriter.Write([]byte("help\n\n")) // Push output
	var lastL3InterfaceID string
	var lastL3InterfaceProto string
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
		if !checkDeviceFailure(device, "Malformed IP CIDR address", err) {
			continue
		}

		// Skip localhost and link-local
		if ipAddress.IsLoopback() || ipAddress.IsLinkLocalUnicast() {
			continue
		}

		// TODO store
		log.WithFields(log.Fields{
			"device":       device.Address,
			"l3_interface": l3Interface,
			"ip_network":   ipNetwork,
		}).Trace("Found L3 interface network")
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
		}).Trace("Found ARP entry")
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
		address := net.ParseIP(rawIPAddress)
		if address == nil {
			showDeviceFailure(device, "Malformed IP address")
			continue
		}
		macAddress, err := net.ParseMAC(rawMACAddress)
		if !checkDeviceFailure(device, "Malformed MAC address", err) {
			continue
		}

		// Skip link-local
		if address.IsLinkLocalUnicast() {
			continue
		}

		// TODO store
		log.WithFields(log.Fields{
			"device":       device.Address,
			"address":      address,
			"mac_address":  macAddress,
			"l3_interface": l3Interface,
		}).Trace("Found NDP entry")
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
