package app

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"

	gendiodes "github.com/cloudfoundry/diodes"

	"code.cloudfoundry.org/loggregator/metron/internal/clientpool"
	clientpoolv2 "code.cloudfoundry.org/loggregator/metron/internal/clientpool/v2"
	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v2"
	ingress "code.cloudfoundry.org/loggregator/metron/internal/ingress/v2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
	NewGauge(name, unit string, opts ...metricemitter.MetricOption) *metricemitter.Gauge
}

type AppV2 struct {
	config          *Config
	healthRegistrar *healthendpoint.Registrar
	clientCreds     credentials.TransportCredentials
	serverCreds     credentials.TransportCredentials
	metricClient    MetricClient
}

func NewV2App(
	c *Config,
	r *healthendpoint.Registrar,
	clientCreds credentials.TransportCredentials,
	serverCreds credentials.TransportCredentials,
	metricClient MetricClient,
) *AppV2 {
	return &AppV2{
		config:          c,
		healthRegistrar: r,
		clientCreds:     clientCreds,
		serverCreds:     serverCreds,
		metricClient:    metricClient,
	}
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
	ingressServer := ingress.NewServer(metronAddress, rx, grpc.Creds(a.serverCreds))
	ingressServer.Start()
}

func (a *AppV2) initializePool() *clientpoolv2.ClientPool {
	if a.clientCreds == nil {
		log.Panic("Failed to load TLS client config")
	}

	balancers := []*clientpoolv2.Balancer{
		clientpoolv2.NewBalancer(fmt.Sprintf("%s.%s", a.config.Zone, a.config.DopplerAddr)),
		clientpoolv2.NewBalancer(a.config.DopplerAddr),
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

	fetcher := clientpoolv2.NewSenderFetcher(
		a.healthRegistrar,
		grpc.WithTransportCredentials(a.clientCreds),
		grpc.WithStatsHandler(statsHandler),
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
