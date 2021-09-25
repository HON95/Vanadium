package common

import (
	"time"

	"dev.hon.one/vanadium/util"
	log "github.com/sirupsen/logrus"
)

// PrometheusNamespace - Prometheus metrics namespace.
const PrometheusNamespace = "vanadium"

// ExpectTimeout - Default timeout for expect scripts.
const ExpectTimeout = 10 * time.Second

// ConfigPath - Path to config file.
var ConfigPath = "config.json"

// Config - The config.
var Config = struct {
	HTTPEndpoint          string  `json:"http_endpoint"`
	CredentialsPath       string  `json:"credentials_path"`
	DevicesPath           string  `json:"devices_path"`
	ScrapeIntervalSeconds float64 `json:"scrape_interval"`
}{
	HTTPEndpoint:          ":80",
	CredentialsPath:       "credentials.json",
	DevicesPath:           "devices.json",
	ScrapeIntervalSeconds: 60.0,
}

// LoadConfig - Load configuration file. Defaults to defaults if not called.
func LoadConfig() bool {
	if ConfigPath == "" {
		// Allow no config
		return true
	}

	log.WithFields(log.Fields{
		"config_path": ConfigPath,
	}).Info("Loading config")

	// Load
	if !util.ParseJSONFile(&Config, ConfigPath) {
		return false
	}

	// Validate
	if Config.ScrapeIntervalSeconds <= 0 {
		log.Error("Non-positive scrape interval not allowed")
		return false
	}

	return true
}
