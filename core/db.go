package core

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"dev.hon.one/vanadium/util"
)

// StartDBClient - Start DB client in the background.
func StartDBClient(waitGroup *sync.WaitGroup, shutdown *util.ShutdownChannelDistributor) {
	// Setup shutdown signal and waitgroup
	shutdownChannel := make(chan bool, 1)
	shutdown.AddListener(shutdownChannel)
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()
		defer log.Info("DB client stopped")

		select {
		case <-shutdownChannel:
			return
		}
	}()

	log.Info("DB client started")
}
