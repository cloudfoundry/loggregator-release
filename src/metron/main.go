package main

import (
	"doppler/dopplerservice"
	"doppler/listeners"
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"time"

	"metron/clientpool"
	"metron/clientreader"
	"metron/networkreader"
	"metron/writers/batch"
	"metron/writers/dopplerforwarder"
	"metron/writers/eventmarshaller"
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
	"github.com/pivotal-golang/localip"

	"metron/config"
	"signalmanager"
)

// This is 6061 to not conflict with any other jobs that might have pprof
// running on 6060
const pprofPort = "6061"

var (
	logFilePath    = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	configFilePath = flag.String("config", "config/metron.json", "Location of the Metron config json file")
	debug          = flag.Bool("debug", false, "Debug logging")
)

func main() {
	// Metron is intended to be light-weight so we occupy only one core
	runtime.GOMAXPROCS(1)

	flag.Parse()
	config, err := config.ParseConfig(*configFilePath)
	if err != nil {
		panic(fmt.Errorf("Unable to parse config: %s", err))
	}

	localIp, err := localip.LocalIP()
	if err != nil {
		panic(fmt.Errorf("Unable to resolve own IP address: %s", err))
	}

	log := logger.NewLogger(*debug, *logFilePath, "metron", config.Syslog)

	go func() {
		err := http.ListenAndServe(net.JoinHostPort(localIp, pprofPort), nil)
		if err != nil {
			log.Errorf("Error starting pprof server: %s", err.Error())
		}
	}()

	log.Info("Startup: Setting up the Metron agent")
	marshaller, err := initializeDopplerPool(config, log)
	if err != nil {
		panic(fmt.Errorf("Could not initialize doppler connection pool: %s", err))
	}
	messageTagger := tagger.New(config.Deployment, config.Job, config.Index, marshaller)
	aggregator := messageaggregator.New(messageTagger, log)

	statsStopChan := make(chan struct{})
	initializeMetrics(messageTagger, config, statsStopChan, log)

	dropsondeUnmarshaller := eventunmarshaller.New(aggregator, log)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", config.IncomingUDPPort)
	dropsondeReader, err := networkreader.New(metronAddress, "dropsondeAgentListener", dropsondeUnmarshaller, log)
	if err != nil {
		panic(fmt.Errorf("Failed to listen on %s: %s", metronAddress, err))
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

func initializeDopplerPool(conf *config.Config, logger *gosteno.Logger) (*eventmarshaller.EventMarshaller, error) {
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
	clientPool := clientpool.NewDopplerPool(logger, udpCreator)
	udpForwarder := dopplerforwarder.New(udpWrapper, clientPool, logger)

	var writer eventmarshaller.ByteWriter = udpForwarder

	switch conf.PreferredProtocol {
	case "tcp":
		tcpCreator := clientpool.NewTCPClientCreator(logger, nil)
		tcpWrapper := dopplerforwarder.NewWrapper(logger, string(conf.PreferredProtocol))
		clientPool = clientpool.NewDopplerPool(logger, tcpCreator)
		tcpForwarder := dopplerforwarder.New(tcpWrapper, clientPool, logger)

		tcpBatchInterval := time.Duration(conf.TCPBatchIntervalMilliseconds) * time.Millisecond
		batchWriter, err := batch.NewWriter(tcpForwarder, conf.TCPBatchSizeBytes, tcpBatchInterval, logger)
		if err != nil {
			return nil, err
		}
		writer = batchWriter

	case "tls":
		c := conf.TLSConfig
		tlsConfig, err := listeners.NewTLSConfig(c.CertFile, c.KeyFile, c.CAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.ServerName = "doppler"
		tlsCreator := clientpool.NewTCPClientCreator(logger, tlsConfig)
		tlsWrapper := dopplerforwarder.NewWrapper(logger, string(conf.PreferredProtocol))
		clientPool = clientpool.NewDopplerPool(logger, tlsCreator)
		tlsForwarder := dopplerforwarder.New(tlsWrapper, clientPool, logger)

		tcpBatchInterval := time.Duration(conf.TCPBatchIntervalMilliseconds) * time.Millisecond
		batchWriter, err := batch.NewWriter(tlsForwarder, conf.TCPBatchSizeBytes, tcpBatchInterval, logger)
		if err != nil {
			return nil, err
		}
		writer = batchWriter
	}

	finder := dopplerservice.NewFinder(adapter, conf.LoggregatorDropsondePort, string(conf.PreferredProtocol), conf.Zone, logger)
	finder.Start()
	go func() {
		for {
			clientreader.Read(clientPool, string(conf.PreferredProtocol), finder.Next())
		}
	}()
	marshaller := eventmarshaller.New(logger)
	marshaller.SetWriter(writer)
	return marshaller, nil
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
