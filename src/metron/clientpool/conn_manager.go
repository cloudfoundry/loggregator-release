package clientpool

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"plumbing"
	"sync/atomic"
	"time"
	"unsafe"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type ConnManager struct {
	dopplerAddr string
	conn        unsafe.Pointer
	maxWrites   int64
}

type grpcConn struct {
	uri    string
	client plumbing.DopplerIngestor_PusherClient
	closer io.Closer
	writes int64
}

func NewConnManager(dopplerAddr string, config *tls.Config, maxWrites int64) *ConnManager {
	m := &ConnManager{
		dopplerAddr: dopplerAddr,
		maxWrites:   maxWrites,
	}

	go m.maintainConn(config)
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
		log.Printf("error writing to doppler %s: %s", gRPCConn.uri, err)
		atomic.StorePointer(&m.conn, nil)
		gRPCConn.closer.Close()
		return err
	}

	if atomic.AddInt64(&gRPCConn.writes, 1) >= m.maxWrites {
		log.Printf("recycling connection to doppler %s after %d writes", gRPCConn.uri, m.maxWrites)
		atomic.StorePointer(&m.conn, nil)
		gRPCConn.closer.Close()
	}

	return nil
}

func (m *ConnManager) maintainConn(config *tls.Config) {
	for range time.Tick(50 * time.Millisecond) {
		conn := atomic.LoadPointer(&m.conn)
		if conn != nil && (*grpcConn)(conn) != nil {
			continue
		}

		transportCreds := credentials.NewTLS(config)
		c, err := grpc.Dial(m.dopplerAddr, grpc.WithTransportCredentials(transportCreds))
		if err != nil {
			log.Printf("error dialing doppler %s: %s", m.dopplerAddr, err)
			continue
		}
		client := plumbing.NewDopplerIngestorClient(c)

		log.Printf("successfully connected to doppler %s", m.dopplerAddr)
		pusher, err := client.Pusher(context.Background())
		if err != nil {
			log.Printf("error establishing ingestor stream to %s: %s", m.dopplerAddr, err)
			continue
		}
		log.Printf("successfully established a stream to doppler %s", m.dopplerAddr)

		atomic.StorePointer(&m.conn, unsafe.Pointer(&grpcConn{
			uri:    m.dopplerAddr,
			client: pusher,
			closer: c,
		}))
	}
}
