package common

// Global non-constant variables go here.

// GlobalConfig - Global singleton.
var GlobalConfig = Config{
	HTTPEndpoint:          ":8080",
	CredentialsPath:       "credentials.json",
	DevicesPath:           "devices.json",
	ScrapeIntervalSeconds: 60.0,
}

// GlobalCredentials - List of loaded credentials, identified by some ID.
var GlobalCredentials map[string]Credential

// GlobalDevices - List of loaded devices, addresses must be unique.
var GlobalDevices []Device
