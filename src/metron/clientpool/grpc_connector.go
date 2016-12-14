package clientpool

import (
	"context"
	"fmt"
	"io"
	"log"
	"plumbing"

	"google.golang.org/grpc"
)

type GRPCConnector struct {
	dopplerAddr    string
	zonePrefix     string
	dial           DialFunc
	ingestorClient IngestorClientFunc
	opts           []grpc.DialOption
}

type DialFunc func(string, ...grpc.DialOption) (*grpc.ClientConn, error)

type IngestorClientFunc func(*grpc.ClientConn) plumbing.DopplerIngestorClient

func MakeGRPCConnector(
	dopplerAddr string,
	zonePrefix string,
	df DialFunc,
	cf IngestorClientFunc,
	opts ...grpc.DialOption,
) GRPCConnector {
	return GRPCConnector{
		dopplerAddr:    dopplerAddr,
		zonePrefix:     zonePrefix,
		dial:           df,
		ingestorClient: cf,
		opts:           opts,
	}
}

func (c GRPCConnector) Connect() (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	closer, pusher, err := c.connect(c.zonePrefix + "." + c.dopplerAddr)
	if err != nil {
		return c.connect(c.dopplerAddr)
	}
	return closer, pusher, err
}

func (c GRPCConnector) connect(addr string) (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	conn, err := c.dial(addr, c.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", c, err)
	}
	client := c.ingestorClient(conn)
	log.Printf("successfully connected to doppler %s", c)
	pusher, err := client.Pusher(context.Background())
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
	return fmt.Sprintf("[%s]%s", c.zonePrefix, c.dopplerAddr)
}
