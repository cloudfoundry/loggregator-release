package main

import (
	"doppler/dopplerservice"
	"doppler/listeners"
	"flag"
	"fmt"
	"os"
	"time"

	"metron/clientpool"
	"metron/networkreader"
	"metron/writers/dopplerforwarder"
	"metron/writers/eventunmarshaller"
	"metron/writers/messageaggregator"
	"metron/writers/tagger"

	"logger"
	"metron/eventwriter"
	"runtime"

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/dropsonde/runtime_stats"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"

	"metron/config"
	"metron/writers/picker"
	"profiler"
	"signalmanager"
)

var (
	logFilePath    = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	configFilePath = flag.String("config", "config/metron.json", "Location of the Metron config json file")
	debug          = flag.Bool("debug", false, "Debug logging")
	cpuprofile     = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile     = flag.String("memprofile", "", "write memory profile to this file")
)

func main() {
	// Put os.Exit in a deferred statement so that other defers get executed prior to
	// the os.Exit call.
	exitCode := 0
	defer func() {
		os.Exit(exitCode)
	}()

	// Metron is intended to be light-weight so we occupy only one core
	runtime.GOMAXPROCS(1)

	flag.Parse()
	config, err := config.ParseConfig(*configFilePath)
	if err != nil {
		panic(err)
	}

	log := logger.NewLogger(*debug, *logFilePath, "metron", config.Syslog)
	log.Info("Startup: Setting up the Metron agent")

	profiler := profiler.New(*cpuprofile, *memprofile, 1*time.Second, log)
	profiler.Profile()
	defer profiler.Stop()

	picker, err := initializeDopplerPool(config, log)
	if err != nil {
		log.Errorf("Could not initialize doppler connection pool: %s", err)
		exitCode = -1
		return
	}
	messageTagger := tagger.New(config.Deployment, config.Job, config.Index, picker)
	aggregator := messageaggregator.New(messageTagger, log)

	statsStopChan := make(chan struct{})
	initializeMetrics(messageTagger, config, statsStopChan, log)

	dropsondeUnmarshaller := eventunmarshaller.New(aggregator, log)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", config.IncomingUDPPort)
	dropsondeReader, err := networkreader.New(metronAddress, "dropsondeAgentListener", dropsondeUnmarshaller, log)
	if err != nil {
		log.Errorf("Failed to listen on %s: %s", metronAddress, err)
		exitCode = 1
		return
	}

	log.Info("metron started")
	go dropsondeReader.Start()

	dumpChan := signalmanager.RegisterGoRoutineDumpSignalChannel()
	killChan := signalmanager.RegisterKillSignalChannel()

	for {
		select {
		case <-dumpChan:
			signalmanager.DumpGoRoutine()
		case <-killChan:
			log.Info("Shutting down")
			close(statsStopChan)
			return
		}
	}
}

func initializeDopplerPool(conf *config.Config, logger *gosteno.Logger) (*picker.Picker, error) {
	adapter, err := storeAdapterProvider(conf.EtcdUrls, conf.EtcdMaxConcurrentRequests)
	if err != nil {
		return nil, err
	}
	err = adapter.Connect()
	if err != nil {
		logger.Warnd(map[string]interface{}{
			"error": err.Error(),
		}, "Failed to connect to etcd")
	}

	udpCreator := clientpool.NewUDPClientCreator(logger)
	udpWrapper := dopplerforwarder.NewUDPWrapper([]byte(conf.SharedSecret), logger)
	udpPool := clientpool.NewDopplerPool(logger, udpCreator)
	udpForwarder := dopplerforwarder.New(udpWrapper, udpPool, nil, logger)
	defaultWriter := udpForwarder
	writers := []picker.WeightedByteWriter{udpForwarder}

	var tlsPool *clientpool.DopplerPool
	if conf.PreferredProtocol == "tls" {
		c := conf.TLSConfig
		tlsConfig, err := listeners.NewTLSConfig(c.CertFile, c.KeyFile, c.CAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.ServerName = "doppler"
		tlsCreator := clientpool.NewTLSClientCreator(logger, tlsConfig)
		tlsWrapper := dopplerforwarder.NewTLSWrapper(logger)
		tlsPool = clientpool.NewDopplerPool(logger, tlsCreator)
		tlsForwarder := dopplerforwarder.New(tlsWrapper, tlsPool, nil, logger)
		defaultWriter = tlsForwarder
		writers = append(writers, tlsForwarder)
	}

	picker, err := picker.New(logger, defaultWriter, writers...)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize the doppler picker: %s", err)
	}

	finder := dopplerservice.NewFinder(adapter, conf.LoggregatorDropsondePort, string(conf.PreferredProtocol), conf.Zone, logger)
	finder.Start()
	go func() {
		for {
			event := finder.Next()
			udpPool.SetAddresses(event.UDPDopplers)
			if tlsPool != nil {
				tlsPool.SetAddresses(event.TLSDopplers)
			}
		}
	}()
	return picker, nil
}

func initializeMetrics(messageTagger *tagger.Tagger, config *config.Config, stopChan chan struct{}, logger *gosteno.Logger) {
	metricsAggregator := messageaggregator.New(messageTagger, logger)

	eventWriter := eventwriter.New("MetronAgent", metricsAggregator)
	metricSender := metric_sender.NewMetricSender(eventWriter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(config.MetricBatchIntervalMilliseconds)*time.Millisecond)
	metrics.Initialize(metricSender, metricBatcher)

	stats := runtime_stats.NewRuntimeStats(eventWriter, time.Duration(config.RuntimeStatsIntervalMilliseconds)*time.Millisecond)
	go stats.Run(stopChan)
}

func storeAdapterProvider(urls []string, concurrentRequests int) (storeadapter.StoreAdapter, error) {
	workPool, err := workpool.NewWorkPool(concurrentRequests)
	if err != nil {
		return nil, err
	}

	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: urls,
	}
	etcdAdapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		return nil, err
	}

	return etcdAdapter, nil
}

type metronHealthMonitor struct{}

func (*metronHealthMonitor) Ok() bool {
	return true
}
