package scraping

import (
	"sync"
	"time"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/db"
	"dev.hon.one/vanadium/util"
	log "github.com/sirupsen/logrus"
)

// StartScraper - Start device scraper in background.
func StartScraper(waitGroup *sync.WaitGroup, shutdown *util.ShutdownChannelDistributor) {
	// Setup shutdown signal and waitgroup
	shutdownChannel := make(chan bool, 1)
	if !shutdown.AddListener(shutdownChannel) {
		return
	}
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()
		defer log.Info("Scraper stopped")

		// Scrape immediately
		scrapeAll()

		for {
			select {
			case <-time.Tick(time.Duration(common.GlobalConfig.ScrapeIntervalSeconds) * time.Second):
				scrapeAll()
			case <-shutdownChannel:
				return
			}
		}
	}()

	log.Info("Scraper started")
}

func scrapeAll() {
	log.Trace("Scraping all devices")
	for _, device := range common.GlobalDevices {
		go scrapeSingle(device)
	}
}

func scrapeSingle(device common.Device) {
	log.WithFields(log.Fields{
		"device": device.Address,
	}).Trace("Scraping device")
	startTime := time.Now()

	// Call appropriate scraper
	var success = false
	switch connectionType := device.ConnectionType; connectionType {
	case common.ConnectionTypeLinuxSSH:
		log.Warn("Not yet implemented: ConnectionTypeLinuxSSH")
	case common.ConnectionTypeVyOSSSH:
		success = VyOSSSH(device, startTime)
	case common.ConnectionTypeJunosEXSSH:
		success = JunosEXSSH(device, startTime)
	case common.ConnectionTypeFSOSSSH:
		log.Warn("Not yet implemented: ConnectionTypeFSOSSSH")
	case common.ConnectionTypeTPLinkJetstreamSSH:
		log.Warn("Not yet implemented: ConnectionTypeTPLinkJetstreamSSH")
	}

	// Record time and duration
	duration := time.Since(startTime)
	scrapeEntry := common.ScrapeEntry{
		Time:     startTime,
		Source:   device.Address,
		Duration: duration,
		Success:  success,
	}
	db.StoreScrapeEntry(scrapeEntry)
}
