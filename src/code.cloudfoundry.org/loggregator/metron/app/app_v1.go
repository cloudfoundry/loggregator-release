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

	clientpool "code.cloudfoundry.org/loggregator/metron/internal/clientpool/v1"
	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v1"
	"code.cloudfoundry.org/loggregator/metron/internal/health"
	ingress "code.cloudfoundry.org/loggregator/metron/internal/ingress/v1"
)

type AppV1 struct {
	config         *Config
	networkReader  *ingress.NetworkReader
	creds          credentials.TransportCredentials
	healthRegistry *health.Registry
}

func NewV1App(c *Config, r *health.Registry, creds credentials.TransportCredentials) *AppV1 {
	statsStopChan := make(chan struct{})
	batcher, eventWriter := initializeMetrics(statsStopChan, c)

	a := &AppV1{
		config:         c,
		healthRegistry: r,
		creds:          creds,
	}

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
		log.Panicf("Failed to listen on %s: %s", metronAddress, err)
	}
	a.networkReader = networkReader

	return a
}

func (a *AppV1) Addr() net.Addr {
	return a.networkReader.Addr()
}

func (a *AppV1) Start() {
	if a.config.DisableUDP {
		return
	}

	log.Printf("metron v1 API started on addr %s", a.Addr())
	go a.networkReader.StartReading()
	a.networkReader.StartWriting()
}

func initializeMetrics(stopChan chan struct{}, config *Config) (*metricbatcher.MetricBatcher, *egress.EventWriter) {
	eventWriter := egress.New("MetronAgent")
	metricSender := metric_sender.NewMetricSender(eventWriter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(config.MetricBatchIntervalMilliseconds)*time.Millisecond)
	metrics.Initialize(metricSender, metricBatcher)

	stats := runtime_stats.NewRuntimeStats(eventWriter, time.Duration(config.RuntimeStatsIntervalMilliseconds)*time.Millisecond)
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

	fetcher := clientpool.NewPusherFetcher(
		a.healthRegistry,
		grpc.WithTransportCredentials(a.creds),
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
