package dopplerforwarder

import (
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
)

type ClientPool interface {
	RandomClient() (loggregatorclient.Client, error)
}

type DopplerForwarder struct {
	clientPool ClientPool
	logger     *gosteno.Logger
}

func New(clientPool ClientPool, logger *gosteno.Logger) *DopplerForwarder {
	return &DopplerForwarder{
		clientPool: clientPool,
		logger:     logger,
	}
}

func (d *DopplerForwarder) Write(message []byte) {
	client, err := d.clientPool.RandomClient()
	if err != nil {
		d.logger.Errorf("can't forward message: %v", err)
		return
	}
	client.Send(message)
	metrics.BatchIncrementCounter("DopplerForwarder.sentMessages")
}
