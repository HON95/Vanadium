package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/db"
	"dev.hon.one/vanadium/http"
	"dev.hon.one/vanadium/scraping"
	"dev.hon.one/vanadium/util"
)

func main() {
	log.Infof("Starting %v version %v by %v", common.AppName, common.AppVersion, common.AppAuthor)

	// Parse CLI args (may exit)
	debug := false
	skipDB := false
	configPath := ""
	flag.BoolVar(&debug, "debug", debug, "Show debug messages.")
	flag.BoolVar(&skipDB, "skip-db", skipDB, "Don't open the DB connection. For testing purposes.")
	flag.StringVar(&configPath, "config", configPath, "Config file path.")
	flag.Parse()
	if debug {
		log.SetLevel(log.TraceLevel)
		log.Info("Debug mode enabled")
	}

	// Load config
	if !common.LoadConfig(configPath) {
		return
	}

	// Load credentials and devices
	if !common.LoadCredentials() || !common.LoadDevices() {
		return
	}

	// Setup internal shutdown mechanism
	osChannel := make(chan os.Signal, 1)
	signal.Notify(osChannel, syscall.SIGINT, syscall.SIGTERM)
	shutdown := util.ShutdownChannelDistributor{}
	go func() {
		osSignal := <-osChannel
		log.Info("Process received signal: ", osSignal)
		shutdown.Shutdown()
	}()

	// Run internal services in background and wait for all to finish
	// Start DB client first to make sure DB is ready for other services
	var waitGroup sync.WaitGroup
	if !skipDB {
		db.StartClient(&waitGroup, &shutdown)
	}
	http.StartServer(&waitGroup, &shutdown)
	scraping.StartScraper(&waitGroup, &shutdown)

	// Wait for internal services to finish
	waitGroup.Wait()
}
