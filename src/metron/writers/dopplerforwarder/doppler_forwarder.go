package dopplerforwarder

import (
	"errors"

	"github.com/cloudfoundry/gosteno"
)

//go:generate hel --type NetworkWrapper --output mock_network_wrapper_test.go

type NetworkWrapper interface {
	Write(client Client, message []byte) error
}

//go:generate hel --type ClientPool --output mock_client_pool_test.go

type ClientPool interface {
	RandomClient() (client Client, err error)
	Size() int
}

// tryLock is a type of lock that returns an error if it is already in use.
// It is named tryLock at the recommendation of Rob Pike:
// https://github.com/golang/go/issues/6123#issuecomment-66083838
type tryLock chan struct{}

func newTryLock() tryLock {
	return make(tryLock, 1)
}

func (t tryLock) Lock() error {
	select {
	case t <- struct{}{}:
		return nil
	default:
		return errors.New("tryLock: lock already acquired")
	}
}

func (t tryLock) Unlock() {
	<-t
}

type DopplerForwarder struct {
	networkWrapper NetworkWrapper
	clientPool     ClientPool
	logger         *gosteno.Logger
	tryLock        tryLock
}

func New(wrapper NetworkWrapper, clientPool ClientPool, logger *gosteno.Logger) *DopplerForwarder {
	return &DopplerForwarder{
		networkWrapper: wrapper,
		clientPool:     clientPool,
		logger:         logger,
		tryLock:        newTryLock(),
	}
}

func (d *DopplerForwarder) Write(message []byte) (int, error) {
	if err := d.tryLock.Lock(); err != nil {
		return 0, errors.New("DopplerForwarder: Write called while write was in progress")
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
	err = d.networkWrapper.Write(client, message)
	if err != nil {
		d.logger.Errord(map[string]interface{}{
			"error": err.Error(),
		}, "DopplerForwarder: failed to write message")

		return 0, err
	}
	return len(message), nil
}
