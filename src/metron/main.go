package main

import (
	"flag"
	"fmt"
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
	"github.com/cloudfoundry/gosteno"

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

	"logger"
	"signalmanager"
)

const (
	origin            = "MetronAgent"
	connectionRetries = 15
	TCPTimeout        = time.Minute
)

func main() {
	logFilePath := flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	configFilePath := flag.String("config", "config/metron.json", "Location of the Metron config json file")
	debug := flag.Bool("debug", false, "Debug logging")

	// Metron is intended to be light-weight so we occupy only one core
	runtime.GOMAXPROCS(1)

	flag.Parse()
	config, err := config.ParseConfig(*configFilePath)
	if err != nil {
		panic(fmt.Errorf("Unable to parse config: %s", err))
	}

	logger := logger.NewLogger(*debug, *logFilePath, "metron", config.Syslog)

	statsStopChan := make(chan struct{})
	batcher, eventWriter := initializeMetrics(config, statsStopChan, logger)

	logger.Info("Startup: Setting up the Metron agent")
	marshaller, err := initializeDopplerPool(config, batcher, logger)
	if err != nil {
		panic(fmt.Errorf("Could not initialize doppler connection pool: %s", err))
	}

	messageTagger := tagger.New(config.Deployment, config.Job, config.Index, marshaller)
	aggregator := messageaggregator.New(messageTagger, logger)
	eventWriter.SetWriter(aggregator)

	dropsondeUnmarshaller := eventunmarshaller.New(aggregator, batcher, logger)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", config.IncomingUDPPort)
	dropsondeReader, err := networkreader.New(metronAddress, "dropsondeAgentListener", dropsondeUnmarshaller, logger)
	if err != nil {
		panic(fmt.Errorf("Failed to listen on %s: %s", metronAddress, err))
	}

	logger.Info("metron started")
	go dropsondeReader.Start()

	dumpChan := signalmanager.RegisterGoRoutineDumpSignalChannel()
	killChan := signalmanager.RegisterKillSignalChannel()

	// We start the profiler last so that we can definitively say that we're all connected and ready
	// for data by the time the profiler starts up.
	p := profiler.New(config.PPROFPort, logger)
	go p.Start()

	for {
		select {
		case <-dumpChan:
			signalmanager.DumpGoRoutine()
		case <-killChan:
			logger.Info("Shutting down")
			close(statsStopChan)
			return
		}
	}
}

func initializeDopplerPool(conf *config.Config, batcher *metricbatcher.MetricBatcher, logger *gosteno.Logger) (*eventmarshaller.EventMarshaller, error) {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		return nil, err
	}

	dialer := clientpool.NewDialer(
		conf.DopplerAddr,
		"",
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)

	var connManagers []clientpool.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpool.NewConnManager(dialer, 10000))
	}

	pool := clientpool.New(connManagers...)
	grpcWrapper := dopplerforwarder.NewGRPCWrapper(pool)

	// TODO: delete this legacy pool stuff when UDP goes away
	legacyPool := legacyclientpool.New(conf.DopplerAddrUDP, 100)
	udpWrapper := dopplerforwarder.NewUDPWrapper(legacyPool, []byte(conf.SharedSecret))

	combinedPool := legacyclientpool.NewCombinedPool(grpcWrapper, udpWrapper)

	marshaller := eventmarshaller.New(batcher, logger)
	marshaller.SetWriter(combinedPool)

	return marshaller, nil
}

func initializeMetrics(config *config.Config, stopChan chan struct{}, logger *gosteno.Logger) (*metricbatcher.MetricBatcher, *eventwriter.EventWriter) {
	eventWriter := eventwriter.New(origin)
	metricSender := metric_sender.NewMetricSender(eventWriter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(config.MetricBatchIntervalMilliseconds)*time.Millisecond)
	metrics.Initialize(metricSender, metricBatcher)

	stats := runtime_stats.NewRuntimeStats(eventWriter, time.Duration(config.RuntimeStatsIntervalMilliseconds)*time.Millisecond)
	go stats.Run(stopChan)
	return metricBatcher, eventWriter
}
