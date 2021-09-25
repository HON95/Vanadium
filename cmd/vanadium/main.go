package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/core"
)

func main() {
	log.WithFields(log.Fields{
		"name":    core.AppName,
		"version": core.AppVersion,
		"author":  core.AppAuthor,
	}).Info("Starting")

	// Parse CLI args (may exit)
	var debug = false
	flag.BoolVar(&debug, "debug", debug, "Show debug messages.")
	flag.StringVar(&common.ConfigPath, "config", common.ConfigPath, "Config file path.")
	flag.Parse()
	if debug {
		log.SetLevel(log.TraceLevel)
		log.Info("Debug mode enabled")
	}

	// Load config
	if !common.LoadConfig() {
		return
	}

	// Load credentials and devices
	if !common.LoadCredentials() || !common.LoadDevices() {
		return
	}

	// Run web server and scraper
	go core.RunHTTPServer()
	go core.RunScraper()

	// Wait for shutdown signal
	signalChannel := make(chan os.Signal, 1)
	doneChannel := make(chan bool, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		receivedSignal := <-signalChannel
		log.WithFields(log.Fields{
			"signal": receivedSignal,
		}).Info("Received shutdown signal")
		// TODO trigger some shutdown logic for web server and scraper here?
		doneChannel <- true
	}()
	<-doneChannel
}
