package v1

import (
	"context"
	"fmt"
	"io"
	"log"
	"plumbing"

	"google.golang.org/grpc"
)

type PusherFetcher struct {
	opts []grpc.DialOption
}

func NewPusherFetcher(opts ...grpc.DialOption) *PusherFetcher {
	return &PusherFetcher{
		opts: opts,
	}
}

func (p *PusherFetcher) Fetch(addr string) (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	conn, err := grpc.Dial(addr, p.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", addr, err)
	}

	client := plumbing.NewDopplerIngestorClient(conn)
	log.Printf("successfully connected to doppler %s", addr)

	pusher, err := client.Pusher(context.Background())
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("error establishing ingestor stream to %s: %s", addr, err)
	}

	log.Printf("successfully established a stream to doppler %s", addr)

	return conn, pusher, err
}
