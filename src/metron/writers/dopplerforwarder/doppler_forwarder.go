package dopplerforwarder

import (
	"errors"
	"fmt"
	"sync"

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
	networkWrapper       NetworkWrapper
	clientPool           ClientPool
	tryLock              tryLock
	congestedDopplerLock sync.Mutex
	congestedDopplers    []string
	logger               *gosteno.Logger
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
		return 0, err
	}
	defer d.tryLock.Unlock()

	client, err := d.clientPool.RandomClient()
	if err != nil {
		d.logger.Errord(map[string]interface{}{
			"error": err.Error(),
		}, "DopplerForwarder: failed to pick a client")
		return 0, err
	}

	var dopplers []string
	d.congestedDopplerLock.Lock()
	d.congestedDopplers = append(d.congestedDopplers, client.Address())
	dopplers = d.congestedDopplers
	d.congestedDopplerLock.Unlock()

	d.logger.Debugf("Writing %d bytes\n", len(message))
	err = d.networkWrapper.Write(client, message, chainers...)
	if err != nil {
		d.logger.Errord(map[string]interface{}{
			"error": err.Error(),
		}, "DopplerForwarder: failed to write message")

		return 0, &ForwarderError{Reason: err.Error()}
	}
	return len(message), nil
}

type ForwarderError struct {
	Reason           string
	CongestedDoppler string
}

func (f ForwarderError) Error() string {
	return fmt.Sprintf("Failed to forward logs: %s", f.Reason)
}

func (w *ForwarderError) DopplerIP() []string {
	return nil
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
		return errors.New("DopplerForwarder: Write already in use")
	}
}

func (t tryLock) Unlock() {
	<-t
}
