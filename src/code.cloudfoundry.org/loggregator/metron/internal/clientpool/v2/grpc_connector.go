package v2

import (
	"errors"
	"io"

	plumbing "code.cloudfoundry.org/loggregator/plumbing/v2"
)

type ClientFetcher interface {
	Fetch(addr string) (conn io.Closer, client plumbing.DopplerIngress_BatchSenderClient, err error)
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

func (c GRPCConnector) Connect() (io.Closer, plumbing.DopplerIngress_BatchSenderClient, error) {
	for _, balancer := range c.balancers {
		hostPort, err := balancer.NextHostPort()
		if err != nil {
			continue
		}

		return c.fetcher.Fetch(hostPort)
	}

	return nil, nil, errors.New("unable to lookup a log consumer")
}
