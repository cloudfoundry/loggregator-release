package app

import (
	"fmt"
	"log"
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

type Metron struct {
	config *Config
	lookup func(string) ([]net.IP, error)
}

// MetronOption configures metron options.
type MetronOption func(*Metron)

// WithLookup allows the default DNS resolver to be changed.
func WithLookup(l func(string) ([]net.IP, error)) func(*Metron) {
	return func(a *Metron) {
		a.lookup = l
	}
}

func NewMetron(
	c *Config,
	opts ...MetronOption,
) *Metron {
	a := &Metron{
		config: c,
		lookup: net.LookupIP,
	}

	for _, o := range opts {
		o(a)
	}

	return a
}

func (a *Metron) Start() {
	clientCreds, err := plumbing.NewClientCredentials(
		a.config.GRPC.CertFile,
		a.config.GRPC.KeyFile,
		a.config.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for client: %s", err)
	}

	var opts []plumbing.ConfigOption
	if len(a.config.GRPC.CipherSuites) > 0 {
		opts = append(opts, plumbing.WithCipherSuites(a.config.GRPC.CipherSuites))
	}

	serverCreds, err := plumbing.NewServerCredentials(
		a.config.GRPC.CertFile,
		a.config.GRPC.KeyFile,
		a.config.GRPC.CAFile,
		opts...,
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	metricsCreds, err := plumbing.NewClientCredentials(
		a.config.GRPC.CertFile,
		a.config.GRPC.KeyFile,
		a.config.GRPC.CAFile,
		"metron",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	batchInterval := time.Duration(a.config.MetricBatchIntervalMilliseconds) * time.Millisecond
	// metric-documentation-v2: setup function
	metricClient, err := metricemitter.NewClient(
		fmt.Sprintf("localhost:%d", a.config.GRPC.Port),
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(metricsCreds)),
		metricemitter.WithOrigin("loggregator.metron"),
		metricemitter.WithDeployment(a.config.Deployment, a.config.Job, a.config.Index),
		metricemitter.WithPulseInterval(batchInterval),
	)
	if err != nil {
		log.Fatalf("Could not configure metric emitter: %s", err)
	}

	healthRegistrar := startHealthEndpoint(fmt.Sprintf(":%d", a.config.HealthEndpointPort))

	appV1 := NewV1App(a.config, healthRegistrar, clientCreds, metricClient)
	go appV1.Start()

	appV2 := NewV2App(a.config, healthRegistrar, clientCreds, serverCreds, metricClient)
	go appV2.Start()
}

func startHealthEndpoint(addr string) *healthendpoint.Registrar {
	promRegistry := prometheus.NewRegistry()
	healthendpoint.StartServer(addr, promRegistry)
	healthRegistrar := healthendpoint.New(promRegistry, map[string]prometheus.Gauge{
		// metric-documentation-health: (dopplerConnections)
		// Number of connections open to dopplers.
		"dopplerConnections": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "metron",
				Name:      "dopplerConnections",
				Help:      "Number of connections open to dopplers",
			},
		),
		// metric-documentation-health: (dopplerV1Streams)
		// Number of V1 gRPC streams to dopplers.
		"dopplerV1Streams": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "metron",
				Name:      "dopplerV1Streams",
				Help:      "Number of V1 gRPC streams to dopplers",
			},
		),
		// metric-documentation-health: (dopplerV2Streams)
		// Number of V2 gRPC streams to dopplers.
		"dopplerV2Streams": prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "loggregator",
				Subsystem: "metron",
				Name:      "dopplerV2Streams",
				Help:      "Number of V2 gRPC streams to dopplers",
			},
		),
	})

	return healthRegistrar
}
