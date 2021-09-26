package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/core"
	"dev.hon.one/vanadium/util"
)

func main() {
	log.Infof("Starting %v version %v by %v", common.AppName, common.AppVersion, common.AppAuthor)

	// Parse CLI args (may exit)
	debug := false
	configPath := ""
	flag.BoolVar(&debug, "debug", debug, "Show debug messages.")
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
	shutdownChannel := make(chan os.Signal, 1)
	signal.Notify(shutdownChannel, syscall.SIGINT, syscall.SIGTERM)
	shutdown := util.NewShutdownChannelDistributor(shutdownChannel)

	// Run internal services in background and wait for all to finish
	var waitGroup sync.WaitGroup
	core.StartHTTPServer(&waitGroup, shutdown)
	core.StartDBClient(&waitGroup, shutdown)
	core.StartScraper(&waitGroup, shutdown)

	// Wait for internal services to finish
	waitGroup.Wait()
}
