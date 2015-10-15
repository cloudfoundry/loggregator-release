package main

import (
	"flag"
	"fmt"
	"time"

	"metron/networkreader"
	"metron/writers/dopplerforwarder"
	"metron/writers/eventmarshaller"
	"metron/writers/eventunmarshaller"
	"metron/writers/legacyunmarshaller"
	"metron/writers/messageaggregator"
	"metron/writers/signer"
	"metron/writers/tagger"

	"metron/eventwriter"
	"runtime"

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
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
	logger         *gosteno.Logger
)

func main() {
	// Metron is intended to be light-weight so we occupy only one core
	runtime.GOMAXPROCS(1)

	flag.Parse()
	config, err := config.ParseConfig(configFilePath)
	if err != nil {
		panic(err)
	}

	logger = cfcomponent.NewLogger(*debug, *logFilePath, "metron", config.Config)
	logger.Info("Startup: Setting up the Metron agent")

	dopplerClientPool := initializeClientPool(config)

	dopplerForwarder := dopplerforwarder.New(dopplerClientPool, logger)
	byteSigner := signer.New(config.SharedSecret, dopplerForwarder)
	marshaller := eventmarshaller.New(byteSigner, logger)
	messageTagger := tagger.New(config.Deployment, config.Job, config.Index, marshaller)
	aggregator := messageaggregator.New(messageTagger, logger)

	initializeMetrics(byteSigner, config)

	dropsondeUnmarshaller := eventunmarshaller.New(aggregator, logger)
	dropsondeReader := networkreader.New(fmt.Sprintf("localhost:%d", config.DropsondeIncomingMessagesPort), "dropsondeAgentListener", dropsondeUnmarshaller, logger)

	// TODO: remove next four lines when legacy support is removed (or extracted to injector)
	legacyMarshaller := eventmarshaller.New(byteSigner, logger)
	legacyMessageTagger := tagger.New(config.Deployment, config.Job, config.Index, legacyMarshaller)
	legacyUnmarshaller := legacyunmarshaller.New(legacyMessageTagger, logger)
	legacyReader := networkreader.New(fmt.Sprintf("localhost:%d", config.LegacyIncomingMessagesPort), "legacyAgentListener", legacyUnmarshaller, logger)

	go legacyReader.Start()
	dropsondeReader.Start()
}

func initializeClientPool(config *config.Config) *clientpool.LoggregatorClientPool {
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

func initializeMetrics(byteSigner *signer.Signer, config *config.Config) {
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

	return etcdstoreadapter.NewETCDStoreAdapter(urls, workPool)
}

type metronHealthMonitor struct{}

func (*metronHealthMonitor) Ok() bool {
	return true
}
