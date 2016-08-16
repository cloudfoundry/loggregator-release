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
	networkWrapper NetworkWrapper
	clientPool     ClientPool
	dopplerLock    *dopplerLock
	logger         *gosteno.Logger
}

func New(wrapper NetworkWrapper, clientPool ClientPool, logger *gosteno.Logger) *DopplerForwarder {
	return &DopplerForwarder{
		networkWrapper: wrapper,
		clientPool:     clientPool,
		dopplerLock:    newTryLock(),
		logger:         logger,
	}
}

func (d *DopplerForwarder) Write(message []byte, chainers ...metricbatcher.BatchCounterChainer) (int, error) {
	if err := d.dopplerLock.Lock(); err != nil {
		return 0, err
	}
	defer d.dopplerLock.Unlock()

	client, err := d.clientPool.RandomClient()
	if err != nil {
		d.logger.Errord(map[string]interface{}{
			"error": err.Error(),
		}, "DopplerForwarder: failed to pick a client")
		return 0, err
	}

	d.dopplerLock.SetAddr(client.Address())
	defer d.dopplerLock.UnsetAddr()

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

type ForwarderError struct {
	reason           string
	congestedDoppler string
}

func (f ForwarderError) Error() string {
	return fmt.Sprintf("DopplerForwarder: %s", f.reason)
}

func (w ForwarderError) CongestedDoppler() string {
	return w.congestedDoppler
}

// dopplerLock is a type of lock that keeps track of doppler
// congestion.
type dopplerLock struct {
	lock     chan struct{}
	addrLock sync.RWMutex
	addr     string
}

func newTryLock() *dopplerLock {
	return &dopplerLock{
		lock: make(chan struct{}, 1),
	}
}

// Addr returns the address of the doppler currently holding
// the lock.  If the address has not yet been set, it will
// block until SetAddr or UnlockAddr is called.
func (t *dopplerLock) Addr() string {
	t.addrLock.RLock()
	defer t.addrLock.RUnlock()
	return t.addr
}

// SetAddr sets the address of the doppler holding the lock.
func (t *dopplerLock) SetAddr(doppler string) {
	t.addrLock.Lock()
	defer t.addrLock.Unlock()
	t.addr = doppler
}

// UnsetAddr sets the address back to an empty string.
func (t *dopplerLock) UnsetAddr() {
	t.addrLock.Lock()
	defer t.addrLock.Unlock()
	t.addr = ""
}

// Lock will try to acquire a lock on t, returning an error if
// the lock has already been acquired.  The returned error will
// be of type ForwarderError if the address of the doppler is
// non-empty.
func (t *dopplerLock) Lock() error {
	select {
	case t.lock <- struct{}{}:
		return nil
	default:
		addr := t.Addr()
		if addr != "" {
			return ForwarderError{reason: "Write already in use", congestedDoppler: addr}
		}
		return errors.New("DopplerForwarder: Write selecting doppler")
	}
}

func (t *dopplerLock) Unlock() {
	<-t.lock
}
