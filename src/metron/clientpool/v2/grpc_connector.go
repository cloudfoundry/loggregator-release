package v2

import (
	"context"
	"fmt"
	"io"
	"log"
	plumbing "plumbing/v2"

	"google.golang.org/grpc"
)

type GRPCConnector struct {
	doppler        string
	dopplerWithAZ  string
	dial           DialFunc
	ingestorClient SenderClientFunc
	opts           []grpc.DialOption
}

type DialFunc func(string, ...grpc.DialOption) (*grpc.ClientConn, error)

type SenderClientFunc func(*grpc.ClientConn) plumbing.DopplerIngressClient

func MakeGRPCConnector(
	doppler string,
	dopplerWithAZ string,
	df DialFunc,
	cf SenderClientFunc,
	opts ...grpc.DialOption,
) GRPCConnector {
	return GRPCConnector{
		doppler:        doppler,
		dopplerWithAZ:  dopplerWithAZ,
		dial:           df,
		ingestorClient: cf,
		opts:           opts,
	}
}

func (c GRPCConnector) Connect() (io.Closer, plumbing.DopplerIngress_SenderClient, error) {
	closer, pusher, err := c.connect(c.dopplerWithAZ)
	if err != nil {
		return c.connect(c.doppler)
	}
	return closer, pusher, err
}

func (c GRPCConnector) connect(doppler string) (io.Closer, plumbing.DopplerIngress_SenderClient, error) {
	conn, err := c.dial(doppler, c.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", c, err)
	}
	client := c.ingestorClient(conn)
	log.Printf("successfully connected to doppler %s", c)
	pusher, err := client.Sender(context.Background())
	if err != nil {
		// TODO: this close is not tested as we don't know how to assert
		// against a grpc.ClientConn being closed.
		conn.Close()
		return nil, nil, fmt.Errorf("error establishing ingestor stream to %s: %s", c, err)
	}
	log.Printf("successfully established a stream to doppler %s", c)

	return conn, pusher, err
}

func (c GRPCConnector) String() string {
	return fmt.Sprintf("[%s]%s", c.dopplerWithAZ, c.doppler)
}
