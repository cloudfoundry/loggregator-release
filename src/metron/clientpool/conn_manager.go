package clientpool

import (
	"context"
	"errors"
	"io"
	"log"
	"plumbing"
	"sync/atomic"
	"time"
	"unsafe"

	"google.golang.org/grpc"
)

type Finder interface {
	Doppler() (uri string)
}

type ConnManager struct {
	finder    Finder
	conn      unsafe.Pointer
	maxWrites int64
}

type grpcConn struct {
	uri    string
	client plumbing.DopplerIngestor_PusherClient
	closer io.Closer
	writes int64
}

func NewConnManager(maxWrites int64, finder Finder) *ConnManager {
	m := &ConnManager{
		finder:    finder,
		maxWrites: maxWrites,
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

func (m *ConnManager) maintainConn() {
	for range time.Tick(50 * time.Millisecond) {
		conn := atomic.LoadPointer(&m.conn)
		if conn != nil && (*grpcConn)(conn) != nil {
			continue
		}

		dopplerURI := m.finder.Doppler()

		c, err := grpc.Dial(dopplerURI, grpc.WithInsecure())
		if err != nil {
			log.Printf("error dialing doppler %s: %s", dopplerURI, err)
			continue
		}
		client := plumbing.NewDopplerIngestorClient(c)

		log.Printf("successfully connected to doppler %s", dopplerURI)
		pusher, err := client.Pusher(context.Background())
		if err != nil {
			log.Printf("error establishing ingestor stream to %s: %s", dopplerURI, err)
			continue
		}
		log.Printf("successfully established a stream to doppler %s", dopplerURI)

		atomic.StorePointer(&m.conn, unsafe.Pointer(&grpcConn{
			uri:    dopplerURI,
			client: pusher,
			closer: c,
		}))
	}
}
