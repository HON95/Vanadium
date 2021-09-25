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

// LoadedCredentials - List of loaded credentials, identified by some ID.
var LoadedCredentials map[string]Credential

// LoadedDevices - List of loaded devices, addresses must be unique.
var LoadedDevices []Device

// LoadCredentials - Load credentials from file from config.
func LoadCredentials() bool {
	if Config.CredentialsPath == "" {
		log.Error("Credentials config path missing")
		return false
	}

	log.WithFields(log.Fields{
		"credentials_path": Config.CredentialsPath,
	}).Trace("Loading credentials")
	if !util.ParseJSONFile(&LoadedCredentials, Config.CredentialsPath) {
		return false
	}

	for credentialID, credential := range LoadedCredentials {
		if credentialID == "" || credential.Username == "" {
			log.WithFields(log.Fields{
				"credential_id":       credentialID,
				"credential_username": credential.Username,
			}).Error("Invalid credential, missing fields")
			return false
		}
	}

	log.WithFields(log.Fields{
		"credential_count": len(LoadedCredentials),
	}).Info("Loaded credentials")

	return true
}

// LoadDevices - Load devices from file from config.
func LoadDevices() bool {
	if Config.DevicesPath == "" {
		log.Error("Device config path missing")
		return false
	}

	log.WithFields(log.Fields{
		"devices_path": Config.DevicesPath,
	}).Trace("Loading devices")
	if !util.ParseJSONFile(&LoadedDevices, Config.DevicesPath) {
		return false
	}

	deviceAddresses := make(map[string]bool)
	for _, device := range LoadedDevices {
		if device.Address == "" || device.CredentialID == "" {
			log.WithFields(log.Fields{
				"device_address": device.Address,
			}).Error("Invalid device, missing fields")
			return false
		}
		// Check for duplicate address
		if _, found := deviceAddresses[device.Address]; found {
			log.WithFields(log.Fields{
				"device_address": device.Address,
			}).Error("Duplicate device address found")
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
			log.WithFields(log.Fields{
				"device_address":  device.Address,
				"connection_type": device.ConnectionType,
			}).Error("Invalid device, connection type not found")
			return false
		}
		// Check if credential ID exists
		if _, found := LoadedCredentials[device.CredentialID]; !found {
			log.WithFields(log.Fields{
				"device_address": device.Address,
				"credential_id":  device.CredentialID,
			}).Error("Invalid device, credential ID not found")
			return false
		}
	}

	log.WithFields(log.Fields{
		"device_count": len(LoadedDevices),
	}).Info("Loaded devices")

	return true
}
