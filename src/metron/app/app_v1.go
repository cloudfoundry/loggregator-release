package app

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/runtime_stats"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	clientpool "metron/internal/clientpool/v1"
	egress "metron/internal/egress/v1"
	"metron/internal/health"
	ingress "metron/internal/ingress/v1"
)

type AppV1 struct {
	config         *Config
	creds          credentials.TransportCredentials
	healthRegistry *health.Registry
}

func NewV1App(c *Config, r *health.Registry, creds credentials.TransportCredentials) *AppV1 {
	return &AppV1{config: c, healthRegistry: r, creds: creds}
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

	balancers := []*clientpool.Balancer{
		clientpool.NewBalancer(fmt.Sprintf("%s.%s", a.config.Zone, a.config.DopplerAddr)),
		clientpool.NewBalancer(a.config.DopplerAddr),
	}

	kp := keepalive.ClientParameters{
		Time:                15 * time.Second,
		Timeout:             15 * time.Second,
		PermitWithoutStream: true,
	}

	fetcher := clientpool.NewPusherFetcher(
		a.healthRegistry,
		grpc.WithTransportCredentials(a.creds),
		grpc.WithKeepaliveParams(kp),
	)

	connector := clientpool.MakeGRPCConnector(fetcher, balancers)

	var connManagers []clientpool.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpool.NewConnManager(
			connector,
			100000+rand.Int63n(1000),
			time.Second,
		))
	}

	pool := clientpool.New(connManagers...)
	return egress.NewGRPCWrapper(pool)
}
