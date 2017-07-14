package v2

import (
	"context"
	"fmt"
	"io"
	"log"

	plumbing "code.cloudfoundry.org/loggregator/plumbing/v2"

	"google.golang.org/grpc"
)

type HealthRegistrar interface {
	Inc(name string)
	Dec(name string)
}

type SenderFetcher struct {
	opts   []grpc.DialOption
	health HealthRegistrar
}

func NewSenderFetcher(r HealthRegistrar, opts ...grpc.DialOption) *SenderFetcher {
	return &SenderFetcher{
		opts:   opts,
		health: r,
	}
}

func (p *SenderFetcher) Fetch(addr string) (io.Closer, plumbing.DopplerIngress_BatchSenderClient, error) {
	conn, err := grpc.Dial(addr, p.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", addr, err)
	}
	p.health.Inc("dopplerConnections")

	client := plumbing.NewDopplerIngressClient(conn)
	log.Printf("successfully connected to doppler %s", addr)

	sender, err := client.BatchSender(context.Background())
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("error establishing ingestor stream to %s: %s", addr, err)
	}
	p.health.Inc("dopplerV2Streams")

	log.Printf("successfully established a stream to doppler %s", addr)

	closer := &decrementingCloser{
		closer: conn,
		health: p.health,
	}
	return closer, sender, err
}

type decrementingCloser struct {
	closer io.Closer
	health HealthRegistrar
}

func (d *decrementingCloser) Close() error {
	d.health.Dec("dopplerConnections")
	d.health.Dec("dopplerV2Streams")

	return d.closer.Close()
}
