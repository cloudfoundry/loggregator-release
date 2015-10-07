package main

import (
	"flag"
	"fmt"
	"time"

	"metron/networkreader"
	"metron/writers/dopplerforwarder"
	"metron/writers/eventmarshaller"
	"metron/writers/eventunmarshaller"
	"metron/writers/messageaggregator"
	"metron/writers/signer"
	"metron/writers/tagger"

	"logger"
	"metron/eventwriter"
	"runtime"

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/clientpool"
	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"

	"metron/config"
)

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
		panic(err)
	}

	log := logger.NewLogger(*debug, *logFilePath, "metron", config.Syslog)
	log.Info("Startup: Setting up the Metron agent")

	dopplerClientPool := initializeClientPool(config, log)

	dopplerForwarder := dopplerforwarder.New(dopplerClientPool, log)
	byteSigner := signer.New(config.SharedSecret, dopplerForwarder)
	marshaller := eventmarshaller.New(byteSigner, log)
	messageTagger := tagger.New(config.Deployment, config.Job, config.Index, marshaller)
	aggregator := messageaggregator.New(messageTagger, log)

	initializeMetrics(byteSigner, config, log)

	dropsondeUnmarshaller := eventunmarshaller.New(aggregator, log)
	dropsondeReader := networkreader.New(fmt.Sprintf("localhost:%d", config.DropsondeIncomingMessagesPort), "dropsondeAgentListener", dropsondeUnmarshaller, log)

	dropsondeReader.Start()
}

func initializeClientPool(config *config.Config, logger *gosteno.Logger) *clientpool.LoggregatorClientPool {
	adapter := storeAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	err := adapter.Connect()
	if err != nil {
		logger.Errorf("Error connecting to ETCD: %v", err)
	}

	inZoneServerAddressList := servicediscovery.NewServerAddressList(adapter, "/healthstatus/doppler/"+config.Zone, logger)
	allZoneServerAddressList := servicediscovery.NewServerAddressList(adapter, "/healthstatus/doppler/", logger)

	go inZoneServerAddressList.Run()
	go allZoneServerAddressList.Run()

	clientPool := clientpool.NewLoggregatorClientPool(logger, config.LoggregatorDropsondePort, inZoneServerAddressList, allZoneServerAddressList)
	return clientPool
}

func initializeMetrics(byteSigner *signer.Signer, config *config.Config, logger *gosteno.Logger) {
	metricsMarshaller := eventmarshaller.New(byteSigner, logger)
	metricsTagger := tagger.New(config.Deployment, config.Job, config.Index, metricsMarshaller)
	metricsAggregator := messageaggregator.New(metricsTagger, logger)

	eventWriter := eventwriter.New("MetronAgent", metricsAggregator)
	metricSender := metric_sender.NewMetricSender(eventWriter)
	metricBatcher := metricbatcher.New(metricSender, time.Duration(config.MetricBatchIntervalSeconds)*time.Second)
	metrics.Initialize(metricSender, metricBatcher)
}

func storeAdapterProvider(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workPool, err := workpool.NewWorkPool(concurrentRequests)
	if err != nil {
		panic(err)
	}

	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: urls,
	}
	etcdAdapter, err := etcdstoreadapter.New(options, workPool)
	if err != nil {
		panic(err)
	}

	return etcdAdapter
}

type metronHealthMonitor struct{}

func (*metronHealthMonitor) Ok() bool {
	return true
}
