package v2

import (
	"context"
	"fmt"
	"io"
	"log"
	plumbing "plumbing/v2"

	"google.golang.org/grpc"
)

type SenderFetcher struct {
	opts []grpc.DialOption
}

func NewSenderFetcher(opts ...grpc.DialOption) *SenderFetcher {
	return &SenderFetcher{
		opts: opts,
	}
}

func (p *SenderFetcher) Fetch(addr string) (io.Closer, plumbing.DopplerIngress_BatchSenderClient, error) {
	conn, err := grpc.Dial(addr, p.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", addr, err)
	}

	client := plumbing.NewDopplerIngressClient(conn)
	log.Printf("successfully connected to doppler %s", addr)

	sender, err := client.BatchSender(context.Background())
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("error establishing ingestor stream to %s: %s", addr, err)
	}

	log.Printf("successfully established a stream to doppler %s", addr)

	return conn, sender, err
}
