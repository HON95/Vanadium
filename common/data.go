package common

import "time"

// ScrapeEntry - Result of a single scrape.
type ScrapeEntry struct {
	// Time the scrape started
	Time time.Time
	// Scraped device address
	Source string
	// Total duration of the scrape
	Duration time.Duration
	// If it didn't fail for any reason
	Success bool
}

// SourceDeviceEntry - Info about a scraped device.
type SourceDeviceEntry struct {
	// Time the scrape started
	Time time.Time
	// Scraped device address
	Source string
	// Device vendor
	Vendor string
	// Device model
	Model string
	// Software name and version
	Software string
	// Hardware vendor/version etc.
	Other string
}

// L2DeviceEntry - L2/switch info about devices on the same LAN.
type L2DeviceEntry struct {
	// Time the scrape started
	Time time.Time
	// Scraped device address
	Source     string
	MACAddress string
	VLANID     uint
	VLANName   string
	// L2 port (e.g. physical)
	L2Interface string
	// If the MAC address points to the source device itself
	IsSelf bool
}

// L3DeviceEntry - L3/router info about devices on the same network/subnet.
type L3DeviceEntry struct {
	// Time the scrape started
	Time time.Time
	// Scraped device address
	Source     string
	IPAddress  string
	MACAddress string
	// L3 port (e.g. VLAN)
	L3Interface string
	// Connected IP network
	IPNetwork string
}
