package dopplerforwarder

import (
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/gosteno"
)

//go:generate hel --type NetworkWrapper --output mock_network_wrapper_test.go

type NetworkWrapper interface {
	Write(client Client, message []byte, chainers ...metricbatcher.BatchCounterChainer) error
}

//go:generate hel --type ClientPool --output mock_client_pool_test.go

type ClientPool interface {
	RandomClient() (client Client, err error)
	Size() int
}

type DopplerForwarder struct {
	networkWrapper NetworkWrapper
	clientPool     ClientPool
	logger         *gosteno.Logger
}

func New(wrapper NetworkWrapper, clientPool ClientPool, logger *gosteno.Logger) *DopplerForwarder {
	return &DopplerForwarder{
		networkWrapper: wrapper,
		clientPool:     clientPool,
		logger:         logger,
	}
}

func (d *DopplerForwarder) Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) (int, error) {
	client, err := d.clientPool.RandomClient()
	if err != nil {
		d.logger.Errord(map[string]interface{}{
			"error": err.Error(),
		}, "DopplerForwarder: failed to pick a client")
		return 0, err
	}
	d.logger.Debugf("Writing %d bytes\n", len(message))
	err = d.networkWrapper.Write(client, message, chainers...)
	if err != nil {
		d.logger.Errord(map[string]interface{}{
			"error": err.Error(),
		}, "DopplerForwarder: failed to write message")

		return 0, err
	}
	return len(message), nil
}
