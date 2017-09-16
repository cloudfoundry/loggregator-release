// package listeners glues several gRPC pieces together. There are no unit
// tests... You have been warned.
package listeners

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1"
	"code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v2"
	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	plumbingv1 "code.cloudfoundry.org/loggregator/plumbing"
	plumbingv2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

type GRPCListener struct {
	listener net.Listener
	server   *grpc.Server

	mu      sync.Mutex
	stopped bool
}

type Batcher interface {
	BatchCounter(name string) metricbatcher.BatchCounterChainer
	BatchIncrementCounter(name string)
	BatchAddCounter(name string, delta uint64)
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
}

type GRPCConfig struct {
	Port         uint16
	CAFile       string
	CertFile     string
	KeyFile      string
	CipherSuites []string
}

func NewGRPCListener(
	reg v1.Registrar,
	envelopeStore v1.EnvelopeStore,
	conf GRPCConfig,
	envelopeBuffer *diodes.ManyToOneEnvelope,
	batcher *metricbatcher.MetricBatcher,
	metricClient MetricClient,
	health *healthendpoint.Registrar,
) (*GRPCListener, error) {
	var opts []plumbingv1.ConfigOption
	if len(conf.CipherSuites) > 0 {
		opts = append(opts, plumbingv1.WithCipherSuites(conf.CipherSuites))
	}

	tlsConfig, err := plumbingv1.NewServerMutualTLSConfig(
		conf.CertFile,
		conf.KeyFile,
		conf.CAFile,
		opts...,
	)
	if err != nil {
		return nil, err
	}
	transportCreds := credentials.NewTLS(tlsConfig)

	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		log.Printf("Failed to start listener (port=%d) for gRPC: %s", conf.Port, err)
		return nil, err
	}
	log.Printf("grpc bound to: %s", grpcListener.Addr())

	kp := keepalive.EnforcementPolicy{
		MinTime:             10 * time.Second,
		PermitWithoutStream: true,
	}
	grpcServer := grpc.NewServer(grpc.Creds(transportCreds), grpc.KeepaliveEnforcementPolicy(kp))

	// v1 ingress
	plumbingv1.RegisterDopplerIngestorServer(
		grpcServer,
		v1.NewIngestorServer(envelopeBuffer, batcher, health),
	)
	// v1 egress
	plumbingv1.RegisterDopplerServer(
		grpcServer,
		v1.NewDopplerServer(reg, envelopeStore, metricClient, health, time.Second, 100),
	)

	// v2 ingress
	plumbingv2.RegisterDopplerIngressServer(
		grpcServer,
		v2.NewIngressServer(envelopeBuffer, batcher, metricClient, health),
	)

	return &GRPCListener{
		listener: grpcListener,
		server:   grpcServer,
	}, nil
}

func (g *GRPCListener) Start() {
	log.Printf("Starting gRPC server on %s", g.listener.Addr().String())
	if err := g.server.Serve(g.listener); err != nil {
		g.mu.Lock()
		stopped := g.stopped
		g.mu.Unlock()

		if !stopped {
			log.Fatalf("Failed to start gRPC server: %s", err)
		}
	}
}

func (g *GRPCListener) Stop() {
	g.mu.Lock()
	g.stopped = true
	g.mu.Unlock()
	g.server.Stop()
}

func (g *GRPCListener) Addr() string {
	return g.listener.Addr().String()
}
