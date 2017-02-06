package v1

import (
	"errors"
	"fmt"
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
	name   string
	client plumbing.DopplerIngestor_PusherClient
	closer io.Closer
	writes int64
}

type ConnManager struct {
	conn      unsafe.Pointer
	maxWrites int64
	connector Connector
}

func NewConnManager(c Connector, maxWrites int64) *ConnManager {
	m := &ConnManager{
		maxWrites: maxWrites,
		connector: c,
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
	for range time.Tick(50 * time.Millisecond) {
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
			name:   fmt.Sprintf("%s", m.connector),
			client: pusherClient,
			closer: closer,
		}))
	}
}
