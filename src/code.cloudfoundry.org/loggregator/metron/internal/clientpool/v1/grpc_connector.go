package v1

import (
	"errors"
	"io"

	"code.cloudfoundry.org/loggregator/plumbing"
)

type ClientFetcher interface {
	Fetch(addr string) (conn io.Closer, client plumbing.DopplerIngestor_PusherClient, err error)
}

type GRPCConnector struct {
	fetcher   ClientFetcher
	balancers []*Balancer
}

func MakeGRPCConnector(fetcher ClientFetcher, balancers []*Balancer) GRPCConnector {
	return GRPCConnector{
		fetcher:   fetcher,
		balancers: balancers,
	}
}

func (c GRPCConnector) Connect() (io.Closer, plumbing.DopplerIngestor_PusherClient, error) {
	for _, balancer := range c.balancers {
		hostPort, err := balancer.NextHostPort()
		if err != nil {
			continue
		}

		return c.fetcher.Fetch(hostPort)
	}

	return nil, nil, errors.New("unable to lookup a log consumer")
}
