package v2

import (
	"errors"
	"fmt"
	"io"
	"log"
	plumbing "plumbing/v2"
	"sync/atomic"
	"time"
	"unsafe"
)

type Connector interface {
	Connect() (io.Closer, plumbing.DopplerIngress_SenderClient, error)
}

type v2GRPCConn struct {
	name   string
	client plumbing.DopplerIngress_SenderClient
	closer io.Closer
	writes int64
}

type ConnManager struct {
	conn         unsafe.Pointer
	maxWrites    int64
	pollDuration time.Duration
	connector    Connector
}

func NewConnManager(c Connector, maxWrites int64, pollDuration time.Duration) *ConnManager {
	m := &ConnManager{
		maxWrites:    maxWrites,
		pollDuration: pollDuration,
		connector:    c,
	}
	go m.maintainConn()
	return m
}

func (m *ConnManager) Write(envelope *plumbing.Envelope) error {
	conn := atomic.LoadPointer(&m.conn)
	if conn == nil || (*v2GRPCConn)(conn) == nil {
		return errors.New("no connection to doppler present")
	}

	gRPCConn := (*v2GRPCConn)(conn)
	err := gRPCConn.client.Send(envelope)

	// TODO: This block is untested because we don't know how to
	// induce an error from the stream via the test
	if err != nil {
		log.Printf("error writing to doppler %s: %s", gRPCConn.name, err)
		atomic.StorePointer(&m.conn, nil)
		gRPCConn.closer.Close()
		return err
	}

	if atomic.AddInt64(&gRPCConn.writes, 1) >= m.maxWrites {
		log.Printf("recycling connection to doppler %s after %d writes", gRPCConn.name, m.maxWrites)
		atomic.StorePointer(&m.conn, nil)
		gRPCConn.closer.Close()
	}

	return nil
}

func (m *ConnManager) maintainConn() {
	for range time.Tick(m.pollDuration) {
		conn := atomic.LoadPointer(&m.conn)
		if conn != nil && (*v2GRPCConn)(conn) != nil {
			continue
		}

		closer, pusherClient, err := m.connector.Connect()
		if err != nil {
			log.Printf("error dialing doppler %s: %s", m.connector, err)
			continue
		}

		atomic.StorePointer(&m.conn, unsafe.Pointer(&v2GRPCConn{
			name:   fmt.Sprintf("%s", m.connector),
			client: pusherClient,
			closer: closer,
		}))
	}
}
