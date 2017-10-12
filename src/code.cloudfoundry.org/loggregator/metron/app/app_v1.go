package app

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/runtime_stats"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/metron/internal/clientpool"
	clientpoolv1 "code.cloudfoundry.org/loggregator/metron/internal/clientpool/v1"
	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v1"
	ingress "code.cloudfoundry.org/loggregator/metron/internal/ingress/v1"
	"code.cloudfoundry.org/loggregator/plumbing"
)

type AppV1 struct {
	config          *Config
	creds           credentials.TransportCredentials
	healthRegistrar *healthendpoint.Registrar
	metricClient    MetricClient
	lookup          func(string) ([]net.IP, error)
}

// AppV1Option configures AppV1 options.
type AppV1Option func(*AppV1)

// WithV1Lookup allows the default DNS resolver to be changed.
func WithV1Lookup(l func(string) ([]net.IP, error)) func(*AppV1) {
	return func(a *AppV1) {
		a.lookup = l
	}
}

func NewV1App(
	c *Config,
	r *healthendpoint.Registrar,
	creds credentials.TransportCredentials,
	m MetricClient,
	opts ...AppV1Option,
) *AppV1 {
	a := &AppV1{
		config:          c,
		healthRegistrar: r,
		creds:           creds,
		metricClient:    m,
		lookup:          net.LookupIP,
	}

	for _, o := range opts {
		o(a)
	}

	return a
}

func (a *AppV1) Start() {
	if a.config.DisableUDP {
		return
	}

	statsStopChan := make(chan struct{})
	batcher, eventWriter := a.initializeMetrics(statsStopChan)

	log.Print("Startup: Setting up the Metron agent")
	marshaller := a.initializeV1DopplerPool(batcher)

	messageTagger := egress.NewTagger(
		a.config.Deployment,
		a.config.Job,
		a.config.Index,
		a.config.IP,
		marshaller,
	)
	aggregator := egress.NewAggregator(messageTagger)
	eventWriter.SetWriter(aggregator)

	dropsondeUnmarshaller := ingress.NewUnMarshaller(aggregator, batcher)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", a.config.IncomingUDPPort)
	networkReader, err := ingress.New(metronAddress, "dropsondeAgentListener", dropsondeUnmarshaller)
	if err != nil {
		log.Panic(fmt.Errorf("Failed to listen on %s: %s", metronAddress, err))
	}

	log.Printf("metron v1 API started on addr %s", metronAddress)
	go networkReader.StartReading()
	networkReader.StartWriting()
}

func (a *AppV1) initializeMetrics(stopChan chan struct{}) (*metricbatcher.MetricBatcher, *egress.EventWriter) {
	eventWriter := egress.New("MetronAgent")
	metricSender := metric_sender.NewMetricSender(eventWriter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(a.config.MetricBatchIntervalMilliseconds)*time.Millisecond)
	metrics.Initialize(metricSender, metricBatcher)

	stats := runtime_stats.NewRuntimeStats(eventWriter, time.Duration(a.config.RuntimeStatsIntervalMilliseconds)*time.Millisecond)
	go stats.Run(stopChan)
	return metricBatcher, eventWriter
}

func (a *AppV1) initializeV1DopplerPool(batcher *metricbatcher.MetricBatcher) *egress.EventMarshaller {
	pool := a.setupGRPC()

	marshaller := egress.NewMarshaller(batcher)
	marshaller.SetWriter(pool)

	return marshaller
}

func (a *AppV1) setupGRPC() egress.BatchChainByteWriter {
	if a.creds == nil {
		return nil
	}

	balancers := []*clientpoolv1.Balancer{
		clientpoolv1.NewBalancer(
			a.config.DopplerAddrWithAZ,
			clientpoolv1.WithLookup(a.lookup),
		),
		clientpoolv1.NewBalancer(
			a.config.DopplerAddr,
			clientpoolv1.WithLookup(a.lookup),
		),
	}

	avgEnvelopeSize := a.metricClient.NewGauge("average_envelope", "bytes/minute",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{
			"loggregator": "v1",
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
	fetcher := clientpoolv1.NewPusherFetcher(
		a.healthRegistrar,
		grpc.WithTransportCredentials(a.creds),
		grpc.WithStatsHandler(statsHandler),
		grpc.WithKeepaliveParams(kp),
	)

	connector := clientpoolv1.MakeGRPCConnector(fetcher, balancers)

	var connManagers []clientpoolv1.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpoolv1.NewConnManager(
			connector,
			100000+rand.Int63n(1000),
			time.Second,
		))
	}

	pool := clientpoolv1.New(connManagers...)
	return egress.NewGRPCWrapper(pool)
}
