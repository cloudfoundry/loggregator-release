package clientpool

import (
	"context"
	"fmt"
	"io"
	"log"
	"plumbing"

	"google.golang.org/grpc"
)

type Dialer struct {
	dopplerAddr        string
	zonePrefix         string
	dialFunc           DialFunc
	ingestorClientFunc IngestorClientFunc
	opts               []grpc.DialOption
}

type DialFunc func(string, ...grpc.DialOption) (*grpc.ClientConn, error)

type IngestorClientFunc func(*grpc.ClientConn) plumbing.DopplerIngestorClient

func NewDialer(
	dopplerAddr string,
	zonePrefix string,
	df DialFunc,
	cf IngestorClientFunc,
	opts ...grpc.DialOption,
) Dialer {
	return Dialer{
		dopplerAddr:        dopplerAddr,
		zonePrefix:         zonePrefix,
		dialFunc:           df,
		ingestorClientFunc: cf,
		opts:               opts,
	}
}

func (d Dialer) Dial() (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	closer, pusher, err := d.connect(d.zonePrefix + "." + d.dopplerAddr)
	if err != nil {
		return d.connect(d.dopplerAddr)
	}
	return closer, pusher, err
}

func (d Dialer) connect(addr string) (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	conn, err := d.dialFunc(addr, d.opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error dialing ingestor stream to %s: %s", d, err)
	}
	client := d.ingestorClientFunc(conn)
	log.Printf("successfully connected to doppler %s", d)
	pusher, err := client.Pusher(context.Background())
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("error establishing ingestor stream to %s: %s", d, err)
	}
	log.Printf("successfully established a stream to doppler %s", d)

	return conn, pusher, err
}

func (d Dialer) String() string {
	// todo: return the exact addr we are connecting to, may be zone specific
	return d.dopplerAddr
}
