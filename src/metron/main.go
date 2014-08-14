package main

import (
	"flag"
	"github.com/cloudfoundry/dropsonde/dropsonde_marshaller"
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/clientpool"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"metron/message_aggregator"
	"metron/varz_forwarder"
	"strconv"
	"time"
)

var (
	logFilePath    = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	configFilePath = flag.String("configFile", "config/metron.json", "Location of the Metron config json file")
	debug          = flag.Bool("debug", false, "Debug logging")
)

var StoreAdapterProvider = func(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workerPool := workerpool.NewWorkerPool(concurrentRequests)

	return etcdstoreadapter.NewETCDStoreAdapter(urls, workerPool)
}

func main() {
	flag.Parse()
	config, logger := parseConfig(*debug, *configFilePath, *logFilePath)

	// TODO: delete next two lines when "legacy" format goes away
	legacyMessageListener, legacyMessageChan := agentlistener.NewAgentListener("localhost:"+strconv.Itoa(config.LegacyIncomingMessagesPort), logger, "legacyAgentListener")
	legacyPoolEtcdAdapter, legacyClientPool := initializeClientPool(config, logger, config.LoggregatorLegacyPort)

	dropsondeMessageListener, dropsondeMessageChan := agentlistener.NewAgentListener("localhost:"+strconv.Itoa(config.DropsondeIncomingMessagesPort), logger, "dropsondeAgentListener")
	dropsondePoolEtcdAdapter, dropsondeClientPool := initializeClientPool(config, logger, config.LoggregatorDropsondePort)

	unmarshaller := dropsonde_unmarshaller.NewDropsondeUnmarshaller(logger)
	marshaller := dropsonde_marshaller.NewDropsondeMarshaller(logger)
	varzForwarder := varz_forwarder.NewVarzForwarder()
	messageAggregator := message_aggregator.NewMessageAggregator(logger)

	instrumentables := []instrumentation.Instrumentable{
		// TODO: delete next line when "legacy" format goes away
		legacyMessageListener,
		dropsondeMessageListener,
		unmarshaller,
		varzForwarder,
		messageAggregator,
		marshaller,
	}

	component := InitializeComponent(DefaultRegistrarFactory, config, logger, instrumentables)

	go startMonitoringEndpoints(component, logger)

	// TODO: delete next three lines when "legacy" format goes away
	go legacyMessageListener.Start()
	go legacyClientPool.RunUpdateLoop(legacyPoolEtcdAdapter, "/healthstatus/loggregator/"+config.Zone, nil, time.Duration(config.EtcdQueryIntervalMilliseconds)*time.Millisecond)
	go forwardMessagesToLoggregator(legacyClientPool, legacyMessageChan, logger)

	go dropsondeMessageListener.Start()

	dropsondeEventChan := make(chan *events.Envelope)
	go unmarshaller.Run(dropsondeMessageChan, dropsondeEventChan)

	varzForwardedEventChan := make(chan *events.Envelope)
	go varzForwarder.Run(dropsondeEventChan, varzForwardedEventChan)

	aggregatedDropsondeEventChan := make(chan *events.Envelope)
	go messageAggregator.Run(varzForwardedEventChan, aggregatedDropsondeEventChan)

	reMarshalledMessageChan := make(chan []byte)
	go marshaller.Run(aggregatedDropsondeEventChan, reMarshalledMessageChan)

	signedMessageChan := make(chan ([]byte))
	go signMessages(config.SharedSecret, reMarshalledMessageChan, signedMessageChan)

	go dropsondeClientPool.RunUpdateLoop(dropsondePoolEtcdAdapter, "/healthstatus/loggregator/"+config.Zone, nil, time.Duration(config.EtcdQueryIntervalMilliseconds)*time.Millisecond)

	forwardMessagesToLoggregator(dropsondeClientPool, signedMessageChan, logger)
}

func signMessages(sharedSecret string, dropsondeMessageChan <-chan ([]byte), signedMessageChan chan<- ([]byte)) {
	for message := range dropsondeMessageChan {
		signedMessage := signature.SignMessage(message, []byte(sharedSecret))
		signedMessageChan <- signedMessage
	}
}

func startMonitoringEndpoints(component *cfcomponent.Component, logger *gosteno.Logger) {
	if err := component.StartMonitoringEndpoints(); err != nil {
		component.Logger.Error(err.Error())
	}
}

func initializeClientPool(config Config, logger *gosteno.Logger, port int) (storeadapter.StoreAdapter, *clientpool.LoggregatorClientPool) {
	adapter := StoreAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	err := adapter.Connect()
	if err != nil {
		logger.Errorf("Error connecting to ETCD: %v", err)
	}

	clientPool := clientpool.NewLoggregatorClientPool(logger, port, true)
	return adapter, clientPool
}

type Config struct {
	cfcomponent.Config
	Zone                          string
	Index                         uint
	LegacyIncomingMessagesPort    int
	DropsondeIncomingMessagesPort int
	EtcdUrls                      []string
	EtcdMaxConcurrentRequests     int
	EtcdQueryIntervalMilliseconds int
	LoggregatorLegacyPort         int
	LoggregatorDropsondePort      int
	SharedSecret                  string
}

type MetronHealthMonitor struct{}

func (*MetronHealthMonitor) Ok() bool {
	return true
}

type RegistrarFactory func(mBusClient yagnats.NATSClient, logger *gosteno.Logger) CollectorRegistrar
type CollectorRegistrar interface {
	RegisterWithCollector(cfc cfcomponent.Component) error
}

func DefaultRegistrarFactory(mBusClient yagnats.NATSClient, logger *gosteno.Logger) CollectorRegistrar {
	return collectorregistrar.NewCollectorRegistrar(mBusClient, logger)
}

func InitializeComponent(registrarFactory RegistrarFactory, config Config, logger *gosteno.Logger, instrumentables []instrumentation.Instrumentable) *cfcomponent.Component {
	if len(config.NatsHosts) == 0 {
		logger.Warn("Startup: Did not receive a NATS host - not going to register component")
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSClient, error) {
			return fakeyagnats.New(), nil
		}
	}

	err := config.Validate(logger)
	if err != nil {
		panic(err)
	}

	component, err := cfcomponent.NewComponent(logger, "MetronAgent", config.Index, &MetronHealthMonitor{}, config.VarzPort, []string{config.VarzUser, config.VarzPass}, instrumentables)
	if err != nil {
		panic(err)
	}

	registrar := registrarFactory(config.MbusClient, logger)
	err = registrar.RegisterWithCollector(component)
	if err != nil {
		logger.Warnf("Unable to register with collector. Err: %v.", err)
	}

	return &component
}

func parseConfig(debug bool, configFile, logFilePath string) (Config, *gosteno.Logger) {
	config := Config{}
	err := cfcomponent.ReadConfigInto(&config, configFile)
	if err != nil {
		panic(err)
	}

	logger := cfcomponent.NewLogger(debug, logFilePath, "metron", config.Config)
	logger.Info("Startup: Setting up the Metron agent")

	return config, logger
}

func forwardMessagesToLoggregator(clientPool *clientpool.LoggregatorClientPool, messageChan <-chan []byte, logger *gosteno.Logger) {
	for message := range messageChan {
		client, err := clientPool.RandomClient()
		if err != nil {
			logger.Errorf("can't forward message: %v", err)
			continue
		}
		client.Send(message)
	}
}
