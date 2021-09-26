package util

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// ShutdownChannelDistributor - For letting multiple listeners receive the internal shutdown signal.
type ShutdownChannelDistributor struct {
	inputChannel   <-chan os.Signal
	outputChannels []chan<- bool
}

// NewShutdownChannelDistributor - Constructor.
func NewShutdownChannelDistributor(input <-chan os.Signal) *ShutdownChannelDistributor {
	shutdown := ShutdownChannelDistributor{inputChannel: input}

	go func() {
		for shutdownSignal := range shutdown.inputChannel {
			log.Infof("Received shutdown signal '%v', sending shutdown signal to %v listeners", shutdownSignal, len(shutdown.outputChannels))
			for _, output := range shutdown.outputChannels {
				output <- true
			}
			break
		}
	}()

	return &shutdown
}

// AddListener - Add a channel to duplicate input to.
func (distributor *ShutdownChannelDistributor) AddListener(output chan<- bool) {
	distributor.outputChannels = append(distributor.outputChannels, output)
	log.Trace("Added new shutdown listener")
	log.Trace("DEBUG", len(distributor.outputChannels)) // DEBUG
}
