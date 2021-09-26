package common

import (
	"dev.hon.one/vanadium/util"
	log "github.com/sirupsen/logrus"
)

// PrometheusNamespace - Prometheus metrics namespace.
const PrometheusNamespace = "vanadium"

// Config - The config.
type Config struct {
	HTTPEndpoint          string  `json:"http_endpoint"`
	CredentialsPath       string  `json:"credentials_path"`
	DevicesPath           string  `json:"devices_path"`
	ScrapeIntervalSeconds float64 `json:"scrape_interval"`
}

// LoadConfig - Load configuration file. Defaults to defaults if not called.
func LoadConfig(configPath string) bool {
	if configPath == "" {
		// Use defaults
		return true
	}

	log.Infof("Loading config: %v", configPath)

	// Load
	if !util.ParseJSONFile(&GlobalConfig, configPath) {
		return false
	}

	// Validate
	if GlobalConfig.ScrapeIntervalSeconds <= 0 {
		log.Error("Non-positive scrape interval not allowed")
		return false
	}

	return true
}
