package dopplerforwarder

import (
	"errors"

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
	tryLock        tryLock
	logger         *gosteno.Logger
}

func New(wrapper NetworkWrapper, clientPool ClientPool, logger *gosteno.Logger) *DopplerForwarder {
	return &DopplerForwarder{
		networkWrapper: wrapper,
		clientPool:     clientPool,
		tryLock:        newTryLock(),
		logger:         logger,
	}
}

func (d *DopplerForwarder) Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) (int, error) {
	if err := d.tryLock.Lock(); err != nil {
		return 0, errors.New("DopplerForwarder: Write already in use")
	}
	defer d.tryLock.Unlock()

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

type tryLock chan struct{}

func newTryLock() tryLock {
	return make(tryLock, 1)
}

func (t tryLock) Lock() error {
	select {
	case t <- struct{}{}:
		return nil
	default:
		return errors.New("Lock in use")
	}
}

func (t tryLock) Unlock() {
	<-t
}
