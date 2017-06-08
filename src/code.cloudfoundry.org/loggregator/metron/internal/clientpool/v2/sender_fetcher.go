package v2

import (
	"code.cloudfoundry.org/loggregator/metron/internal/health"
	"context"
	"fmt"
	"io"
	"log"
	plumbing "plumbing/v2"

	"google.golang.org/grpc"
)

type HealthRegistry interface {
	RegisterValue(name string) *health.Value
}

type SenderFetcher struct {
	opts        []grpc.DialOption
	connValue   *health.Value
	streamValue *health.Value
}

func NewSenderFetcher(r HealthRegistry, opts ...grpc.DialOption) *SenderFetcher {
	return &SenderFetcher{
		opts:        opts,
		connValue:   r.RegisterValue("doppler_connections"),
		streamValue: r.RegisterValue("doppler_v2_streams"),
	}
}

func (p *SenderFetcher) Fetch(addr string) (io.Closer, plumbing.DopplerIngress_BatchSenderClient, error) {
	conn, err := grpc.Dial(addr, p.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", addr, err)
	}
	p.connValue.Increment(1)

	client := plumbing.NewDopplerIngressClient(conn)
	log.Printf("successfully connected to doppler %s", addr)

	sender, err := client.BatchSender(context.Background())
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
	return closer, sender, err
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
