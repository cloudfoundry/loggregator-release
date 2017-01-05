package main

import (
	"flag"
	"fmt"
	"log"
	"plumbing"
	"profiler"
	"runtime"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/runtime_stats"

	"metron/clientpool"
	"metron/config"
	"metron/eventwriter"
	"metron/legacyclientpool"
	"metron/networkreader"
	"metron/writers/dopplerforwarder"
	"metron/writers/eventmarshaller"
	"metron/writers/eventunmarshaller"
	"metron/writers/messageaggregator"
	"metron/writers/tagger"

	"signalmanager"
)

const (
	origin            = "MetronAgent"
	connectionRetries = 15
	TCPTimeout        = time.Minute
)

func main() {
	configFilePath := flag.String("config", "config/metron.json", "Location of the Metron config json file")

	// Metron is intended to be light-weight so we occupy only one core
	runtime.GOMAXPROCS(1)

	flag.Parse()
	config, err := config.ParseConfig(*configFilePath)
	if err != nil {
		panic(fmt.Errorf("Unable to parse config: %s", err))
	}

	statsStopChan := make(chan struct{})
	batcher, eventWriter := initializeMetrics(config, statsStopChan)

	log.Print("Startup: Setting up the Metron agent")
	marshaller, err := initializeDopplerPool(config, batcher)
	if err != nil {
		panic(fmt.Errorf("Could not initialize doppler connection pool: %s", err))
	}

	messageTagger := tagger.New(config.Deployment, config.Job, config.Index, marshaller)
	aggregator := messageaggregator.New(messageTagger)
	eventWriter.SetWriter(aggregator)

	dropsondeUnmarshaller := eventunmarshaller.New(aggregator, batcher)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", config.IncomingUDPPort)
	dropsondeReader, err := networkreader.New(metronAddress, "dropsondeAgentListener", dropsondeUnmarshaller)
	if err != nil {
		panic(fmt.Errorf("Failed to listen on %s: %s", metronAddress, err))
	}

	log.Print("metron started")
	go dropsondeReader.Start()

	dumpChan := signalmanager.RegisterGoRoutineDumpSignalChannel()
	killChan := signalmanager.RegisterKillSignalChannel()

	// We start the profiler last so that we can definitively say that we're all connected and ready
	// for data by the time the profiler starts up.
	p := profiler.New(config.PPROFPort)
	go p.Start()

	for {
		select {
		case <-dumpChan:
			signalmanager.DumpGoRoutine()
		case <-killChan:
			log.Print("Shutting down")
			close(statsStopChan)
			return
		}
	}
}

func setupGRPC(conf *config.Config) []legacyclientpool.Pool {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		log.Printf("Failed to load TLS config: %s", err)
		return nil
	}

	dialer := clientpool.MakeGRPCConnector(
		conf.DopplerAddr,
		conf.Zone,
		grpc.Dial,
		plumbing.NewDopplerIngestorClient,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)

	var connManagers []clientpool.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpool.NewConnManager(dialer, 10000))
	}

	pool := clientpool.New(connManagers...)
	grpcWrapper := dopplerforwarder.NewGRPCWrapper(pool)
	return []legacyclientpool.Pool{grpcWrapper}
}

func initializeDopplerPool(conf *config.Config, batcher *metricbatcher.MetricBatcher) (*eventmarshaller.EventMarshaller, error) {
	pools := setupGRPC(conf)

	// TODO: delete this legacy pool stuff when UDP goes away
	legacyPool := legacyclientpool.New(conf.DopplerAddrUDP, 100, 5*time.Second)
	udpWrapper := dopplerforwarder.NewUDPWrapper(legacyPool, []byte(conf.SharedSecret))
	pools = append(pools, udpWrapper)

	combinedPool := legacyclientpool.NewCombinedPool(pools...)

	marshaller := eventmarshaller.New(batcher)
	marshaller.SetWriter(combinedPool)

	return marshaller, nil
}

func initializeMetrics(config *config.Config, stopChan chan struct{}) (*metricbatcher.MetricBatcher, *eventwriter.EventWriter) {
	eventWriter := eventwriter.New(origin)
	metricSender := metric_sender.NewMetricSender(eventWriter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(config.MetricBatchIntervalMilliseconds)*time.Millisecond)
	metrics.Initialize(metricSender, metricBatcher)

	stats := runtime_stats.NewRuntimeStats(eventWriter, time.Duration(config.RuntimeStatsIntervalMilliseconds)*time.Millisecond)
	go stats.Run(stopChan)
	return metricBatcher, eventWriter
}
