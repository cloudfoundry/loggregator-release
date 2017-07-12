package v2

import (
	"errors"
	"io"
	"log"
	"sync/atomic"
	"time"
	"unsafe"

	plumbing "code.cloudfoundry.org/loggregator/plumbing/v2"
)

type Connector interface {
	Connect() (io.Closer, plumbing.DopplerIngress_BatchSenderClient, error)
}

type v2GRPCConn struct {
	client plumbing.DopplerIngress_BatchSenderClient
	closer io.Closer
	writes int64
}

type ConnManager struct {
	conn         unsafe.Pointer
	maxWrites    int64
	pollDuration time.Duration
	connector    Connector

	ticker *time.Ticker
	reset  chan bool
}

func NewConnManager(c Connector, maxWrites int64, pollDuration time.Duration) *ConnManager {
	m := &ConnManager{
		maxWrites:    maxWrites,
		pollDuration: pollDuration,
		connector:    c,
		ticker:       time.NewTicker(pollDuration),
		reset:        make(chan bool, 100),
	}
	go m.maintainConn()
	return m
}

func (m *ConnManager) Write(envelopes []*plumbing.Envelope) error {
	conn := atomic.LoadPointer(&m.conn)
	if conn == nil || (*v2GRPCConn)(conn) == nil {
		return errors.New("no connection to doppler present")
	}

	gRPCConn := (*v2GRPCConn)(conn)
	err := gRPCConn.client.Send(&plumbing.EnvelopeBatch{Batch: envelopes})

	if err != nil {
		log.Printf("error writing to doppler: %s", err)
		atomic.StorePointer(&m.conn, nil)
		gRPCConn.closer.Close()
		m.reset <- true
		return err
	}

	if atomic.AddInt64(&gRPCConn.writes, 1) >= m.maxWrites {
		log.Printf("recycling connection to doppler after %d writes", m.maxWrites)
		atomic.StorePointer(&m.conn, nil)
		gRPCConn.closer.Close()
		m.reset <- true
	}

	return nil
}

func (m *ConnManager) maintainConn() {

	// Ensure initial connection does not wait on timer
	m.reset <- true

	for {
		m.checkConnectionTimer()

		conn := atomic.LoadPointer(&m.conn)
		if conn != nil && (*v2GRPCConn)(conn) != nil {
			continue
		}

		closer, senderClient, err := m.connector.Connect()
		if err != nil {
			continue
		}

		atomic.StorePointer(&m.conn, unsafe.Pointer(&v2GRPCConn{
			client: senderClient,
			closer: closer,
		}))
	}
}

func (m *ConnManager) checkConnectionTimer() {
	select {
	case <-m.ticker.C:
	case <-m.reset:
	}
}
