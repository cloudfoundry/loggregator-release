package app

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"

	gendiodes "code.cloudfoundry.org/diodes"

	"code.cloudfoundry.org/loggregator/metron/internal/clientpool"
	clientpoolv2 "code.cloudfoundry.org/loggregator/metron/internal/clientpool/v2"
	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v2"
	ingress "code.cloudfoundry.org/loggregator/metron/internal/ingress/v2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
	NewGauge(name, unit string, opts ...metricemitter.MetricOption) *metricemitter.Gauge
}

// AppV2Option configures AppV2 options.
type AppV2Option func(*AppV2)

// WithV2Lookup allows the default DNS resolver to be changed.
func WithV2Lookup(l func(string) ([]net.IP, error)) func(*AppV2) {
	return func(a *AppV2) {
		a.lookup = l
	}
}

type AppV2 struct {
	config          *Config
	healthRegistrar *healthendpoint.Registrar
	clientCreds     credentials.TransportCredentials
	serverCreds     credentials.TransportCredentials
	metricClient    MetricClient
	lookup          func(string) ([]net.IP, error)
}

func NewV2App(
	c *Config,
	r *healthendpoint.Registrar,
	clientCreds credentials.TransportCredentials,
	serverCreds credentials.TransportCredentials,
	metricClient MetricClient,
	opts ...AppV2Option,
) *AppV2 {
	a := &AppV2{
		config:          c,
		healthRegistrar: r,
		clientCreds:     clientCreds,
		serverCreds:     serverCreds,
		metricClient:    metricClient,
		lookup:          net.LookupIP,
	}

	for _, o := range opts {
		o(a)
	}

	return a
}

func (a *AppV2) Start() {
	if a.serverCreds == nil {
		log.Panic("Failed to load TLS server config")
	}

	droppedMetric := a.metricClient.NewCounter("dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{"direction": "ingress"}),
	)

	envelopeBuffer := diodes.NewManyToOneEnvelopeV2(10000, gendiodes.AlertFunc(func(missed int) {
		// metric-documentation-v2: (loggregator.metron.dropped) Number of v2 envelopes
		// dropped from the metron ingress diode
		droppedMetric.Increment(uint64(missed))

		log.Printf("Dropped %d v2 envelopes", missed)
	}))

	pool := a.initializePool()
	counterAggr := egress.NewCounterAggregator(pool)
	tx := egress.NewTransponder(
		envelopeBuffer,
		counterAggr,
		a.config.Tags,
		100, time.Second,
		a.metricClient,
	)
	go tx.Start()

	metronAddress := fmt.Sprintf("127.0.0.1:%d", a.config.GRPC.Port)
	log.Printf("metron v2 API started on addr %s", metronAddress)

	rx := ingress.NewReceiver(envelopeBuffer, a.metricClient)
	kp := keepalive.EnforcementPolicy{
		MinTime:             10 * time.Second,
		PermitWithoutStream: true,
	}
	ingressServer := ingress.NewServer(
		metronAddress,
		rx,
		grpc.Creds(a.serverCreds),
		grpc.KeepaliveEnforcementPolicy(kp),
	)
	ingressServer.Start()
}

func (a *AppV2) initializePool() *clientpoolv2.ClientPool {
	if a.clientCreds == nil {
		log.Panic("Failed to load TLS client config")
	}

	balancers := []*clientpoolv2.Balancer{
		clientpoolv2.NewBalancer(a.config.DopplerAddrWithAZ, clientpoolv2.WithLookup(a.lookup)),
		clientpoolv2.NewBalancer(a.config.DopplerAddr, clientpoolv2.WithLookup(a.lookup)),
	}

	avgEnvelopeSize := a.metricClient.NewGauge("average_envelope", "bytes/minute",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{
			"loggregator": "v2",
		}))
	tracker := plumbing.NewEnvelopeAverager()
	tracker.Start(60*time.Second, func(average float64) {
		avgEnvelopeSize.Set(average)
	})
	statsHandler := clientpool.NewStatsHandler(tracker)

	kp := keepalive.ClientParameters{
		Time:                15 * time.Second,
		Timeout:             15 * time.Second,
		PermitWithoutStream: true,
	}
	fetcher := clientpoolv2.NewSenderFetcher(
		a.healthRegistrar,
		grpc.WithTransportCredentials(a.clientCreds),
		grpc.WithStatsHandler(statsHandler),
		grpc.WithKeepaliveParams(kp),
	)

	connector := clientpoolv2.MakeGRPCConnector(fetcher, balancers)

	var connManagers []clientpoolv2.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpoolv2.NewConnManager(
			connector,
			100000+rand.Int63n(1000),
			time.Second,
		))
	}

	return clientpoolv2.New(connManagers...)
}
