package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"golang.org/x/net/netutil"

	"github.com/prometheus/client_golang/prometheus"

	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"
	v2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"code.cloudfoundry.org/loggregator/rlp/internal/egress"
	"code.cloudfoundry.org/loggregator/rlp/internal/ingress"

	"google.golang.org/grpc"
)

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
}

// RLP represents the reverse log proxy component. It connects to various gRPC
// servers to ingress data and opens a gRPC server to egress data.
type RLP struct {
	ctx       context.Context
	ctxCancel func()

	egressPort           int
	egressServerOpts     []grpc.ServerOption
	maxEgressConnections int

	ingressAddrs    []string
	ingressDialOpts []grpc.DialOption
	ingressPool     *plumbing.Pool

	receiver *ingress.Receiver
	querier  *ingress.Querier

	egressAddr     net.Addr
	egressListener net.Listener
	egressServer   *grpc.Server

	healthAddr string
	health     *healthendpoint.Registrar

	metricClient MetricClient

	finder *plumbing.StaticFinder
}

// NewRLP returns a new unstarted RLP.
func NewRLP(m MetricClient, opts ...RLPOption) *RLP {
	ctx, cancel := context.WithCancel(context.Background())
	rlp := &RLP{
		ingressAddrs:         []string{"doppler.service.cf.internal"},
		ingressDialOpts:      []grpc.DialOption{grpc.WithInsecure()},
		egressServerOpts:     []grpc.ServerOption{},
		maxEgressConnections: 500,
		metricClient:         m,
		healthAddr:           "localhost:33333",
		ctx:                  ctx,
		ctxCancel:            cancel,
	}
	for _, o := range opts {
		o(rlp)
	}

	rlp.startEgressListener()

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

// WithMaxEgressConnections specifies the number of connections the RLP will
// accept on the egress endpoint.
func WithMaxEgressConnections(max int) RLPOption {
	return func(r *RLP) {
		r.maxEgressConnections = max
	}
}

// EgressAddr returns the address used for the egress server.
func (r *RLP) EgressAddr() net.Addr {
	return r.egressAddr
}

// Start starts a remote log proxy. This connects to various gRPC servers and
// listens for gRPC connections for egressing data.
func (r *RLP) Start() {
	r.setupHealthEndpoint()
	r.setupIngress()
	r.setupEgress()
	r.serveEgress()
}

// Stop stops the remote log proxy. It stops listening for new subscriptions
// and drains existing ones. Stop will not return until existing connections
// are drained or timeout has elapsed.
func (r *RLP) Stop() {
	r.ctxCancel()
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()
	go func() {
		defer wg.Done()
		r.egressServer.GracefulStop()
	}()

	// Stop reconnects to ingress servers
	r.finder.Stop()

	// Close current connections to ingress servers
	for _, addr := range r.ingressAddrs {
		r.ingressPool.Close(addr)
	}
}

func (r *RLP) setupIngress() {
	r.finder = plumbing.NewStaticFinder(r.ingressAddrs)
	r.ingressPool = plumbing.NewPool(20, r.ingressDialOpts...)

	batcher := &ingress.NullMetricBatcher{} // TODO: Add real metrics

	connector := plumbing.NewGRPCConnector(1000, r.ingressPool, r.finder, batcher, r.metricClient)
	converter := ingress.NewConverter()
	r.receiver = ingress.NewReceiver(converter, ingress.NewRequestConverter(), connector)
	r.querier = ingress.NewQuerier(converter, connector)
}

func (r *RLP) startEgressListener() {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", r.egressPort))
	if err != nil {
		log.Fatalf("failed to listen on port: %d: %s", r.egressPort, err)
	}
	r.egressListener = netutil.LimitListener(l, r.maxEgressConnections)
	r.egressAddr = r.egressListener.Addr()
}

func (r *RLP) setupEgress() {
	r.egressServer = grpc.NewServer(r.egressServerOpts...)
	v2.RegisterEgressServer(
		r.egressServer,
		egress.NewServer(r.receiver, r.metricClient, r.health, r.ctx),
	)
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
	if err := r.egressServer.Serve(r.egressListener); err != nil && !r.isDone() {
		log.Fatal("failed to serve: ", err)
	}
}

func (r *RLP) isDone() bool {
	select {
	case <-r.ctx.Done():
		return true
	default:
		return false
	}
}
