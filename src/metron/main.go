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

	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/clientpool"
	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"metron/eventwriter"
	"runtime"
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
	config, logger := parseConfig(*debug, *configFilePath, *logFilePath)

	dopplerClientPool := initializeClientPool(config, logger)

	dopplerForwarder := dopplerforwarder.New(dopplerClientPool, logger)
	byteSigner := signer.New(config.SharedSecret, dopplerForwarder)
	marshaller := eventmarshaller.New(byteSigner, logger)
	messageTagger := tagger.New(config.Deployment, config.Job, config.Index, marshaller)
	aggregator := messageaggregator.New(messageTagger, logger)

	initializeMetrics(byteSigner, config, logger)

	dropsondeUnmarshaller := eventunmarshaller.New(aggregator, logger)
	dropsondeReader := networkreader.New(fmt.Sprintf("localhost:%d", config.DropsondeIncomingMessagesPort), "dropsondeAgentListener", dropsondeUnmarshaller, logger)

	// TODO: remove next four lines when legacy support is removed (or extracted to injector)
	legacyMarshaller := eventmarshaller.New(byteSigner, logger)
	legacyMessageTagger := tagger.New(config.Deployment, config.Job, config.Index, legacyMarshaller)
	legacyUnmarshaller := legacyunmarshaller.New(legacyMessageTagger, logger)
	legacyReader := networkreader.New(fmt.Sprintf("localhost:%d", config.LegacyIncomingMessagesPort), "legacyAgentListener", legacyUnmarshaller, logger)

	instrumentables := []instrumentation.Instrumentable{
		legacyReader,
		dropsondeReader,
		legacyUnmarshaller,
		dropsondeUnmarshaller,
		aggregator,
		marshaller,
	}

	go startMonitoringEndpoints(config, instrumentables, logger)

	go legacyReader.Start()
	dropsondeReader.Start()
}

func startMonitoringEndpoints(config metronConfig, instrumentables []instrumentation.Instrumentable, logger *gosteno.Logger) {
	component := initializeComponent(config, instrumentables, logger)
	go collectorregistrar.NewCollectorRegistrar(cfcomponent.DefaultYagnatsClientProvider, component, time.Duration(config.CollectorRegistrarIntervalMilliseconds)*time.Millisecond, &config.Config).Run()

	if err := component.StartMonitoringEndpoints(); err != nil {
		component.Logger.Error(err.Error())
	}
}

func initializeClientPool(config metronConfig, logger *gosteno.Logger) *clientpool.LoggregatorClientPool {
	adapter := storeAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	err := adapter.Connect()
	if err != nil {
		logger.Errorf("Error connecting to ETCD: %v", err)
	}

	inZoneServerAddressList := servicediscovery.NewServerAddressList(adapter, "/healthstatus/doppler/"+config.Zone, logger)
	allZoneServerAddressList := servicediscovery.NewServerAddressList(adapter, "/healthstatus/doppler/", logger)

	go inZoneServerAddressList.Run(time.Duration(config.EtcdQueryIntervalMilliseconds) * time.Millisecond)
	go allZoneServerAddressList.Run(time.Duration(config.EtcdQueryIntervalMilliseconds) * time.Millisecond)

	clientPool := clientpool.NewLoggregatorClientPool(logger, config.LoggregatorDropsondePort, inZoneServerAddressList, allZoneServerAddressList)
	return clientPool
}

func initializeMetrics(byteSigner *signer.Signer, config metronConfig, logger *gosteno.Logger) {
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

func initializeComponent(config metronConfig, instrumentables []instrumentation.Instrumentable, logger *gosteno.Logger) cfcomponent.Component {
	if len(config.NatsHosts) == 0 {
		logger.Warn("Startup: Did not receive a NATS host - not going to register component")
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSConn, error) {
			return fakeyagnats.Connect(), nil
		}
	}

	component, err := cfcomponent.NewComponent(logger, "MetronAgent", config.Index, &metronHealthMonitor{}, config.VarzPort, []string{config.VarzUser, config.VarzPass}, instrumentables)
	if err != nil {
		panic(err)
	}

	return component
}

func parseConfig(debug bool, configFile string, logFilePath string) (metronConfig, *gosteno.Logger) {
	config := metronConfig{}
	err := cfcomponent.ReadConfigInto(&config, configFile)
	if err != nil {
		panic(err)
	}

	if config.MetricBatchIntervalSeconds == 0 {
		config.MetricBatchIntervalSeconds = 15
	}

	logger := cfcomponent.NewLogger(debug, logFilePath, "metron", config.Config)
	logger.Info("Startup: Setting up the Metron agent")

	return config, logger
}

type metronConfig struct {
	cfcomponent.Config

	Deployment string
	Zone       string
	Job        string
	Index      uint

	LegacyIncomingMessagesPort    int
	DropsondeIncomingMessagesPort int

	EtcdUrls                      []string
	EtcdMaxConcurrentRequests     int
	EtcdQueryIntervalMilliseconds int

	LoggregatorDropsondePort int
	SharedSecret             string

	MetricBatchIntervalSeconds uint
}

type metronHealthMonitor struct{}

func (*metronHealthMonitor) Ok() bool {
	return true
}
