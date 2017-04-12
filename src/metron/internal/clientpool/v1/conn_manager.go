package v1

import (
	"errors"
	"io"
	"log"
	"plumbing"
	"sync/atomic"
	"time"
	"unsafe"
)

type Connector interface {
	Connect() (io.Closer, plumbing.DopplerIngestor_PusherClient, error)
}

type grpcConn struct {
	client plumbing.DopplerIngestor_PusherClient
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

func (m *ConnManager) Write(data []byte) error {
	conn := atomic.LoadPointer(&m.conn)
	if conn == nil || (*grpcConn)(conn) == nil {
		return errors.New("no connection to doppler present")
	}

	gRPCConn := (*grpcConn)(conn)
	err := gRPCConn.client.Send(&plumbing.EnvelopeData{
		Payload: data,
	})

	// TODO: This block is untested because we don't know how to
	// induce an error from the stream via the test
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
		if conn != nil && (*grpcConn)(conn) != nil {
			continue
		}

		closer, pusherClient, err := m.connector.Connect()
		if err != nil {
			log.Printf("error dialing doppler %s: %s", m.connector, err)
			continue
		}

		atomic.StorePointer(&m.conn, unsafe.Pointer(&grpcConn{
			client: pusherClient,
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
