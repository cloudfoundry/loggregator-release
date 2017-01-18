package clientpool

import (
	"context"
	"fmt"
	"io"
	"log"
	v2 "plumbing/v2"

	"google.golang.org/grpc"
)

type GRPCV2Connector struct {
	doppler        string
	zonePrefix     string
	dial           DialFunc
	ingestorClient SenderClientFunc
	opts           []grpc.DialOption
}

type SenderClientFunc func(*grpc.ClientConn) v2.DopplerIngressClient

func MakeV2Connector(
	doppler string,
	zonePrefix string,
	df DialFunc,
	cf SenderClientFunc,
	opts ...grpc.DialOption,
) GRPCV2Connector {
	return GRPCV2Connector{
		doppler:        doppler,
		zonePrefix:     zonePrefix,
		dial:           df,
		ingestorClient: cf,
		opts:           opts,
	}
}

func (c GRPCV2Connector) Connect() (io.Closer, v2.DopplerIngress_SenderClient, error) {
	closer, pusher, err := c.connect(c.zonePrefix + "." + c.doppler)
	if err != nil {
		return c.connect(c.doppler)
	}
	return closer, pusher, err
}

func (c GRPCV2Connector) connect(doppler string) (io.Closer, v2.DopplerIngress_SenderClient, error) {
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

func (c GRPCV2Connector) String() string {
	return fmt.Sprintf("[%s]%s", c.zonePrefix, c.doppler)
}
