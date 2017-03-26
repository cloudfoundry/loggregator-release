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

	"metron/internal/clientpool/legacy"
	clientpool "metron/internal/clientpool/v1"
	egress "metron/internal/egress/v1"
	ingress "metron/internal/ingress/v1"
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
	pools := a.setupGRPC()

	// TODO: delete this legacy pool stuff when UDP goes away
	legacyPool := legacy.New(a.config.DopplerAddrUDP, 100, 5*time.Second)
	udpWrapper := egress.NewUDPWrapper(legacyPool, []byte(a.config.SharedSecret))
	pools = append(pools, udpWrapper)

	combinedPool := legacy.NewCombinedPool(pools...)

	marshaller := egress.NewMarshaller(batcher)
	marshaller.SetWriter(combinedPool)

	return marshaller
}

func (a *AppV1) setupGRPC() []legacy.Pool {
	if a.creds == nil {
		return nil
	}

	balancers := []*clientpool.Balancer{
		clientpool.NewBalancer(fmt.Sprintf("%s.%s", a.config.Zone, a.config.DopplerAddr)),
		clientpool.NewBalancer(a.config.DopplerAddr),
	}

	fetcher := clientpool.NewPusherFetcher(
		grpc.WithTransportCredentials(a.creds),
	)

	connector := clientpool.MakeGRPCConnector(fetcher, balancers)

	var connManagers []clientpool.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpool.NewConnManager(
			connector,
			10000+rand.Int63n(1000),
			time.Second,
		))
	}

	pool := clientpool.New(connManagers...)
	grpcWrapper := egress.NewGRPCWrapper(pool)
	return []legacy.Pool{grpcWrapper}
}
