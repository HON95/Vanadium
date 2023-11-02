package util

import (
	log "github.com/sirupsen/logrus"
)

// ShutdownChannelDistributor - For letting multiple listeners receive the internal shutdown signal.
type ShutdownChannelDistributor struct {
	hasShutdown    bool
	outputChannels []chan<- bool
}

// AddListener - Add a channel to duplicate input to.
// Return false if the shutdown signal has already been sent.
func (shutdown *ShutdownChannelDistributor) AddListener(output chan<- bool) bool {
	if shutdown.hasShutdown {
		return false
	}
	shutdown.outputChannels = append(shutdown.outputChannels, output)
	return true
}

// Shutdown - Send shutdown signal to all listeners.
func (shutdown *ShutdownChannelDistributor) Shutdown() {
	shutdown.hasShutdown = true
	log.Infof("Sending shutdown signal to %v listeners", len(shutdown.outputChannels))
	for _, output := range shutdown.outputChannels {
		output <- true
	}
}
