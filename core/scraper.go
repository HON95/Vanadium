package core

import (
	"sync"
	"time"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/scrapers"
	"dev.hon.one/vanadium/util"
	log "github.com/sirupsen/logrus"
)

// StartScraper - Start device scraper in background.
func StartScraper(waitGroup *sync.WaitGroup, shutdown *util.ShutdownChannelDistributor) {
	// Setup shutdown signal and waitgroup
	shutdownChannel := make(chan bool, 1)
	shutdown.AddListener(shutdownChannel)
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
