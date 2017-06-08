package v1

import (
	"code.cloudfoundry.org/loggregator/metron/internal/health"
	"context"
	"fmt"
	"io"
	"log"
	"plumbing"

	"google.golang.org/grpc"
)

type HealthRegistry interface {
	RegisterValue(name string) *health.Value
}

type PusherFetcher struct {
	opts        []grpc.DialOption
	connValue   *health.Value
	streamValue *health.Value
}

func NewPusherFetcher(r HealthRegistry, opts ...grpc.DialOption) *PusherFetcher {
	return &PusherFetcher{
		opts:        opts,
		connValue:   r.RegisterValue("doppler_connections"),
		streamValue: r.RegisterValue("doppler_v1_streams"),
	}
}

func (p *PusherFetcher) Fetch(addr string) (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	conn, err := grpc.Dial(addr, p.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", addr, err)
	}
	p.connValue.Increment(1)

	client := plumbing.NewDopplerIngestorClient(conn)
	log.Printf("successfully connected to doppler %s", addr)

	pusher, err := client.Pusher(context.Background())
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("error establishing ingestor stream to %s: %s", addr, err)
	}
	p.streamValue.Increment(1)

	log.Printf("successfully established a stream to doppler %s", addr)

	closer := &decrementingCloser{
		closer:      conn,
		connValue:   p.connValue,
		streamValue: p.streamValue,
	}
	return closer, pusher, err
}

type decrementingCloser struct {
	closer      io.Closer
	connValue   *health.Value
	streamValue *health.Value
}

func (d *decrementingCloser) Close() error {
	d.connValue.Decrement(1)
	d.streamValue.Decrement(1)

	return d.closer.Close()
}
