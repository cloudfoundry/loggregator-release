package app

import (
	"rlp/internal/ingress"
	"trafficcontroller/grpcconnector"

	"google.golang.org/grpc"
)

func Start(opts ...AppOption) string {
	conf := setupConfig(opts)

	finder := ingress.NewFinder(conf.ingressAddrs)
	pool := grpcconnector.NewPool(20, conf.dialOpts...)

	// TODO Add real metrics
	grpcconnector.New(1000, pool, finder, &ingress.NullMetricBatcher{})

	return ""
}

type AppOption func(c *config)

func WithLogRouterAddrs(addrs []string) func(*config) {
	return func(c *config) {
		c.ingressAddrs = addrs
	}
}

func WithDialOptions(opts ...grpc.DialOption) func(*config) {
	return func(c *config) {
		c.dialOpts = opts
	}
}

type config struct {
	ingressAddrs []string
	dialOpts     []grpc.DialOption
}

func setupConfig(opts []AppOption) *config {
	conf := config{
		ingressAddrs: []string{"doppler.service.cf.internal"},
		dialOpts:     []grpc.DialOption{grpc.WithInsecure()},
	}

	for _, o := range opts {
		o(&conf)
	}

	return &conf
}
