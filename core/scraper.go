package core

import (
	"time"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/scrapers"
	log "github.com/sirupsen/logrus"
)

// RunScraper - Run device scraper.
func RunScraper() {
	scrapeAll()
	for range time.Tick(time.Duration(common.Config.ScrapeIntervalSeconds) * time.Second) {
		scrapeAll()
	}
}

func scrapeAll() {
	for _, device := range common.LoadedDevices {
		go scrapeSingle(device)
	}
}

func scrapeSingle(device common.Device) {
	log.WithFields(log.Fields{
		"device_address": device.Address,
	}).Trace("Scraping device")
	startTime := time.Now()

	// Call appropriate scraper
	var success = false
	switch connectionType := device.ConnectionType; connectionType {
	case common.ConnectionTypeLinuxSSH:
		log.Fatal("Not yet implemented")
	case common.ConnectionTypeVyOSSSH:
		success = scrapers.VyOSSSH(device)
	case common.ConnectionTypeJunosEXSSH:
		success = scrapers.JunosEXSSH(device)
	case common.ConnectionTypeFSOSSSH:
		log.Fatal("Not yet implemented")
	case common.ConnectionTypeTPLinkJetstreamSSH:
		log.Fatal("Not yet implemented")
	}

	// Record time and duration
	duration := time.Since(startTime)
	log.WithFields(log.Fields{
		"scrape_start":    startTime,
		"scrape_duration": duration,
		"scrape_success":  success,
	}).Trace("Scraping device done")
	// TODO scrape result in database
}
