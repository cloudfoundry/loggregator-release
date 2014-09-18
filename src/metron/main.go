package main

import (
	"flag"
	"github.com/cloudfoundry/dropsonde/dropsonde_marshaller"
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/clientpool"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"metron/legacy_message/legacy_message_converter"
	"metron/legacy_message/legacy_unmarshaller"
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

var METRIC_TTL = emitter.HeartbeatInterval * 5

var StoreAdapterProvider = func(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workerPool := workerpool.NewWorkerPool(concurrentRequests)

	return etcdstoreadapter.NewETCDStoreAdapter(urls, workerPool)
}

func main() {
	flag.Parse()
	config, logger := parseConfig(*debug, *configFilePath, *logFilePath)

	// TODO: delete next two lines when "legacy" format goes away
	legacyMessageListener, legacyMessageChan := agentlistener.NewAgentListener("localhost:"+strconv.Itoa(config.LegacyIncomingMessagesPort), logger, "legacyAgentListener")

	dropsondeMessageListener, dropsondeMessageChan := agentlistener.NewAgentListener("localhost:"+strconv.Itoa(config.DropsondeIncomingMessagesPort), logger, "dropsondeAgentListener")
	dropsondePoolEtcdAdapter, dropsondeClientPool := initializeClientPool(config, logger, config.LoggregatorDropsondePort)

	legacyUnmarshaller := legacy_unmarshaller.NewLegacyUnmarshaller(logger)
	legacyMessageConverter := legacy_message_converter.NewLegacyMessageConverter(logger)

	unmarshaller := dropsonde_unmarshaller.NewDropsondeUnmarshaller(logger)
	marshaller := dropsonde_marshaller.NewDropsondeMarshaller(logger)
	varzForwarder := varz_forwarder.NewVarzForwarder(config.Job, METRIC_TTL, logger)
	messageAggregator := message_aggregator.NewMessageAggregator(logger)

	instrumentables := []instrumentation.Instrumentable{
		legacyMessageListener,
		dropsondeMessageListener,
		unmarshaller,
		varzForwarder,
		messageAggregator,
		marshaller,
	}

	component := InitializeComponent(DefaultRegistrarFactory, config, logger, instrumentables)

	go startMonitoringEndpoints(component, logger)
	dropsondeEventChan := make(chan *events.Envelope)

	go legacyMessageListener.Start()

	logEnvelopesChan := make(chan *logmessage.LogEnvelope)
	go legacyUnmarshaller.Run(legacyMessageChan, logEnvelopesChan)
	go legacyMessageConverter.Run(logEnvelopesChan, dropsondeEventChan)

	go dropsondeMessageListener.Start()

	go unmarshaller.Run(dropsondeMessageChan, dropsondeEventChan)

	varzForwardedEventChan := make(chan *events.Envelope)
	go varzForwarder.Run(dropsondeEventChan, varzForwardedEventChan)

	aggregatedDropsondeEventChan := make(chan *events.Envelope)
	go messageAggregator.Run(varzForwardedEventChan, aggregatedDropsondeEventChan)

	reMarshalledMessageChan := make(chan []byte)
	go marshaller.Run(aggregatedDropsondeEventChan, reMarshalledMessageChan)

	signedMessageChan := make(chan ([]byte))
	go signMessages(config.SharedSecret, reMarshalledMessageChan, signedMessageChan)

	go dropsondeClientPool.RunUpdateLoop(dropsondePoolEtcdAdapter, "/healthstatus/doppler/"+config.Zone, nil, time.Duration(config.EtcdQueryIntervalMilliseconds)*time.Millisecond)

	forwardMessagesToDoppler(dropsondeClientPool, signedMessageChan, logger)
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

	clientPool := clientpool.NewLoggregatorClientPool(logger, port)
	return adapter, clientPool
}

type Config struct {
	cfcomponent.Config
	Zone                          string
	Index                         uint
	Job                           string
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

type RegistrarFactory func(mBusClient yagnats.ApceraWrapperNATSClient, logger *gosteno.Logger) CollectorRegistrar
type CollectorRegistrar interface {
	RegisterWithCollector(cfc cfcomponent.Component) error
}

func DefaultRegistrarFactory(mBusClient yagnats.ApceraWrapperNATSClient, logger *gosteno.Logger) CollectorRegistrar {
	return collectorregistrar.NewCollectorRegistrar(mBusClient, logger)
}

func InitializeComponent(registrarFactory RegistrarFactory, config Config, logger *gosteno.Logger, instrumentables []instrumentation.Instrumentable) *cfcomponent.Component {
	if len(config.NatsHosts) == 0 {
		logger.Warn("Startup: Did not receive a NATS host - not going to register component")
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.ApceraWrapperNATSClient, error) {
			return fakeyagnats.NewApceraClientWrapper(), nil
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

func forwardMessagesToDoppler(clientPool *clientpool.LoggregatorClientPool, messageChan <-chan []byte, logger *gosteno.Logger) {
	for message := range messageChan {
		client, err := clientPool.RandomClient()
		if err != nil {
			logger.Errorf("can't forward message: %v", err)
			continue
		}
		client.Send(message)
	}
}
