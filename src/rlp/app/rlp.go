package app

import (
	"fmt"
	"healthendpoint"
	"log"
	"metricemitter"
	"net"

	"github.com/prometheus/client_golang/prometheus"

	"plumbing"
	v2 "plumbing/v2"
	"rlp/internal/egress"
	"rlp/internal/ingress"

	"google.golang.org/grpc"
)

// RLP represents the reverse log proxy component. It connects to various gRPC
// servers to ingress data and opens a gRPC server to egress data.
type RLP struct {
	egressPort       int
	egressServerOpts []grpc.ServerOption

	ingressAddrs    []string
	ingressDialOpts []grpc.DialOption

	receiver *ingress.Receiver
	querier  *ingress.Querier

	egressAddr     net.Addr
	egressListener net.Listener
	egressServer   *grpc.Server

	healthAddr string
	health     *healthendpoint.Registrar

	metricClient metricemitter.MetricClient
}

// NewRLP returns a new unstarted RLP.
func NewRLP(m metricemitter.MetricClient, opts ...RLPOption) *RLP {
	rlp := &RLP{
		ingressAddrs:     []string{"doppler.service.cf.internal"},
		ingressDialOpts:  []grpc.DialOption{grpc.WithInsecure()},
		egressServerOpts: []grpc.ServerOption{},
		metricClient:     m,
		healthAddr:       "localhost:22222",
	}
	for _, o := range opts {
		o(rlp)
	}
	return rlp
}

// RLPOption represents a function that can configure a remote log proxy.
type RLPOption func(c *RLP)

// WithEgressPort specifies the port used to bind the egress gRPC server.
func WithEgressPort(port int) RLPOption {
	return func(r *RLP) {
		r.egressPort = port
	}
}

// WithEgressServerOptions specifies the dial options used when serving data via
// gRPC.
func WithEgressServerOptions(opts ...grpc.ServerOption) RLPOption {
	return func(r *RLP) {
		r.egressServerOpts = opts
	}
}

// WithIngressAddrs specifies the addresses used to connect to ingress data.
func WithIngressAddrs(addrs []string) RLPOption {
	return func(r *RLP) {
		r.ingressAddrs = addrs
	}
}

// WithIngressDialOptions specifies the dial options used when connecting to
// the gRPC server to ingress data.
func WithIngressDialOptions(opts ...grpc.DialOption) RLPOption {
	return func(r *RLP) {
		r.ingressDialOpts = opts
	}
}

// WithHealthAddr specifies the host and port to bind to for servicing the
// health endpoint.
func WithHealthAddr(addr string) RLPOption {
	return func(r *RLP) {
		r.healthAddr = addr
	}
}

// Start starts a remote log proxy. This connects to various gRPC servers and
// listens for gRPC connections for egressing data.
func (r *RLP) Start() {
	r.setupHealthEndpoint()
	r.setupIngress()
	r.setupEgress()
	r.serveEgress()
}

func (r *RLP) setupIngress() {
	finder := plumbing.NewStaticFinder(r.ingressAddrs)
	pool := plumbing.NewPool(20, r.ingressDialOpts...)

	batcher := &ingress.NullMetricBatcher{} // TODO: Add real metrics

	connector := plumbing.NewGRPCConnector(1000, pool, finder, batcher, r.metricClient)
	converter := ingress.NewConverter()
	r.receiver = ingress.NewReceiver(converter, ingress.NewRequestConverter(), connector)
	r.querier = ingress.NewQuerier(converter, connector)
}

func (r *RLP) setupEgress() {
	var err error
	r.egressListener, err = net.Listen("tcp", fmt.Sprintf(":%d", r.egressPort))
	if err != nil {
		log.Fatalf("failed to listen on port: %d: %s", r.egressPort, err)
	}
	r.egressAddr = r.egressListener.Addr()
	r.egressServer = grpc.NewServer(r.egressServerOpts...)
	v2.RegisterEgressServer(
		r.egressServer,
		egress.NewServer(r.receiver, r.metricClient, r.health))
	v2.RegisterEgressQueryServer(r.egressServer, egress.NewQueryServer(r.querier))
}

func (r *RLP) setupHealthEndpoint() {
	promRegistry := prometheus.NewRegistry()
	healthendpoint.StartServer(r.healthAddr, promRegistry)
	r.health = healthendpoint.New(promRegistry, map[string]prometheus.Gauge{
		// metric-documentation-health: (subscriptionCount)
		// Number of open subscriptions
		"subscriptionCount": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "reverseLogProxy",
				Name:      "subscriptionCount",
				Help:      "Number of open subscriptions",
			},
		),
	})
}

func (r *RLP) serveEgress() {
	if err := r.egressServer.Serve(r.egressListener); err != nil {
		log.Fatal("failed to serve: ", err)
	}
}
