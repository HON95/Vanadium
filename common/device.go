package common

import (
	log "github.com/sirupsen/logrus"

	"dev.hon.one/vanadium/util"
)

// Connection types (device type plus protocol).
const (
	ConnectionTypeLinuxSSH           = "linux_ssh"
	ConnectionTypeVyOSSSH            = "vyos_ssh"
	ConnectionTypeJunosEXSSH         = "junos_ex_ssh"
	ConnectionTypeFSOSSSH            = "fsos_ssh"
	ConnectionTypeTPLinkJetstreamSSH = "tplink_jetstream_ssh"
)

// Credential - Credential for a device.
type Credential struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	PrivateKeyPath string `json:"private_key_path"`
}

// Device - A device to scrape.
type Device struct {
	Address        string `json:"address"` // Unique
	Port           uint   `json:"port"`    // Optional, default to normal service port
	ConnectionType string `json:"connection_type"`
	CredentialID   string `json:"credential_id"`
}

// LoadCredentials - Load credentials from file from config.
func LoadCredentials() bool {
	if GlobalConfig.CredentialsPath == "" {
		log.Error("Credentials config path missing")
		return false
	}

	if !util.ParseJSONFile(&GlobalCredentials, GlobalConfig.CredentialsPath) {
		return false
	}

	for credentialID, credential := range GlobalCredentials {
		if credentialID == "" || credential.Username == "" {
			log.Errorf("Invalid credential, missing fields: %v", credentialID)
			return false
		}
	}

	log.Infof("Loaded %v credentials: %v", len(GlobalCredentials), GlobalConfig.CredentialsPath)

	return true
}

// LoadDevices - Load devices from file from config.
func LoadDevices() bool {
	if GlobalConfig.DevicesPath == "" {
		log.Error("Device config path missing")
		return false
	}

	if !util.ParseJSONFile(&GlobalDevices, GlobalConfig.DevicesPath) {
		return false
	}

	deviceAddresses := make(map[string]bool)
	for _, device := range GlobalDevices {
		if device.Address == "" || device.CredentialID == "" {
			log.Errorf("Invalid device, missing fields: %v", device.Address)
			return false
		}
		// Check for duplicate address
		if _, found := deviceAddresses[device.Address]; found {
			log.Errorf("Duplicate device address found: %v", device.Address)
			return false
		}
		deviceAddresses[device.Address] = true
		// Check if connection type exists
		switch connectionType := device.ConnectionType; connectionType {
		case ConnectionTypeLinuxSSH:
		case ConnectionTypeVyOSSSH:
		case ConnectionTypeJunosEXSSH:
		case ConnectionTypeFSOSSSH:
		case ConnectionTypeTPLinkJetstreamSSH:
		default:
			log.Errorf("Invalid device, connection type not found: %v", device.Address)
			return false
		}
		// Check if credential ID exists
		if _, found := GlobalCredentials[device.CredentialID]; !found {
			log.Error("Invalid device, credential ID not found: %v", device.Address)
			return false
		}
	}

	log.Infof("Loaded %v devices: %v", len(GlobalDevices), GlobalConfig.DevicesPath)

	return true
}
