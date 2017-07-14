package v1

import (
	"context"
	"fmt"
	"io"
	"log"

	"code.cloudfoundry.org/loggregator/plumbing"

	"google.golang.org/grpc"
)

type HealthRegistrar interface {
	Inc(name string)
	Dec(name string)
}

type PusherFetcher struct {
	opts   []grpc.DialOption
	health HealthRegistrar
}

func NewPusherFetcher(r HealthRegistrar, opts ...grpc.DialOption) *PusherFetcher {
	return &PusherFetcher{
		opts:   opts,
		health: r,
	}
}

func (p *PusherFetcher) Fetch(addr string) (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	conn, err := grpc.Dial(addr, p.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", addr, err)
	}
	p.health.Inc("dopplerConnections")

	client := plumbing.NewDopplerIngestorClient(conn)
	log.Printf("successfully connected to doppler %s", addr)

	pusher, err := client.Pusher(context.Background())
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("error establishing ingestor stream to %s: %s", addr, err)
	}
	p.health.Inc("dopplerV1Streams")

	log.Printf("successfully established a stream to doppler %s", addr)

	closer := &decrementingCloser{
		closer: conn,
		health: p.health,
	}
	return closer, pusher, err
}

type decrementingCloser struct {
	closer io.Closer
	health HealthRegistrar
}

func (d *decrementingCloser) Close() error {
	d.health.Dec("dopplerConnections")
	d.health.Dec("dopplerV1Streams")

	return d.closer.Close()
}
