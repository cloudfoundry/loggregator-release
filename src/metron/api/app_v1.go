package api

import (
	"fmt"
	"log"
	"math/rand"
	clientpool "metron/clientpool/v1"
	"metron/eventwriter"
	"metron/clientpool/legacy"
	ingress "metron/ingress/v1"
	"metron/writers/dopplerforwarder"
	"metron/writers/eventmarshaller"
	"metron/writers/eventunmarshaller"
	"metron/writers/messageaggregator"
	"metron/writers/tagger"
	"plumbing"
	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/runtime_stats"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type AppV1 struct {
	config *Config
	creds  credentials.TransportCredentials
}

func NewV1App(c *Config, creds credentials.TransportCredentials) *AppV1 {
	return &AppV1{config: c, creds: creds}
}

func (a *AppV1) Start() {
	if a.config.DisableUDP {
		return
	}

	statsStopChan := make(chan struct{})
	batcher, eventWriter := a.initializeMetrics(statsStopChan)

	log.Print("Startup: Setting up the Metron agent")
	marshaller := a.initializeV1DopplerPool(batcher)

	messageTagger := tagger.New(a.config.Deployment, a.config.Job, a.config.Index, marshaller)
	aggregator := messageaggregator.New(messageTagger)
	eventWriter.SetWriter(aggregator)

	dropsondeUnmarshaller := eventunmarshaller.New(aggregator, batcher)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", a.config.IncomingUDPPort)
	dropsondeReader, err := ingress.New(metronAddress, "dropsondeAgentListener", dropsondeUnmarshaller)
	if err != nil {
		log.Panic(fmt.Errorf("Failed to listen on %s: %s", metronAddress, err))
	}

	log.Printf("metron v1 API started on addr %s", metronAddress)
	dropsondeReader.Start()
}

func (a *AppV1) initializeMetrics(stopChan chan struct{}) (*metricbatcher.MetricBatcher, *eventwriter.EventWriter) {
	eventWriter := eventwriter.New("MetronAgent")
	metricSender := metric_sender.NewMetricSender(eventWriter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(a.config.MetricBatchIntervalMilliseconds)*time.Millisecond)
	metrics.Initialize(metricSender, metricBatcher)

	stats := runtime_stats.NewRuntimeStats(eventWriter, time.Duration(a.config.RuntimeStatsIntervalMilliseconds)*time.Millisecond)
	go stats.Run(stopChan)
	return metricBatcher, eventWriter
}

func (a *AppV1) initializeV1DopplerPool(batcher *metricbatcher.MetricBatcher) *eventmarshaller.EventMarshaller {
	pools := a.setupGRPC()

	// TODO: delete this legacy pool stuff when UDP goes away
	legacyPool := legacy.New(a.config.DopplerAddrUDP, 100, 5*time.Second)
	udpWrapper := dopplerforwarder.NewUDPWrapper(legacyPool, []byte(a.config.SharedSecret))
	pools = append(pools, udpWrapper)

	combinedPool := legacy.NewCombinedPool(pools...)

	marshaller := eventmarshaller.New(batcher)
	marshaller.SetWriter(combinedPool)

	return marshaller
}

func (a *AppV1) setupGRPC() []legacy.Pool {
	if a.creds == nil {
		return nil
	}

	connector := clientpool.MakeGRPCConnector(
		a.config.DopplerAddr,
		a.config.Zone,
		grpc.Dial,
		plumbing.NewDopplerIngestorClient,
		grpc.WithTransportCredentials(a.creds),
	)

	var connManagers []clientpool.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpool.NewConnManager(connector, 10000+rand.Int63n(1000)))
	}

	pool := clientpool.New(connManagers...)
	grpcWrapper := dopplerforwarder.NewGRPCWrapper(pool)
	return []legacy.Pool{grpcWrapper}
}
