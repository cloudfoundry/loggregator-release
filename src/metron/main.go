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
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"metron/eventlistener"
	"metron/legacy_message/legacy_message_converter"
	"metron/legacy_message/legacy_unmarshaller"
	"metron/message_aggregator"
	"metron/pingsender"
	"metron/varz_forwarder"
	"strconv"
	"time"
)

var (
	logFilePath    = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	configFilePath = flag.String("configFile", "config/metron.json", "Location of the Metron config json file")
	debug          = flag.Bool("debug", false, "Debug logging")
)

var METRIC_TTL = time.Second * 5
var PING_SENDER_INTERVAL = time.Second * 1

var StoreAdapterProvider = func(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workerPool := workerpool.NewWorkerPool(concurrentRequests)

	return etcdstoreadapter.NewETCDStoreAdapter(urls, workerPool)
}

func main() {
	flag.Parse()
	config, logger := parseConfig(*debug, *configFilePath, *logFilePath)

	// TODO: delete next line when "legacy" format goes away
	legacyMessageListener, legacyMessageChan := agentlistener.NewAgentListener("localhost:"+strconv.Itoa(config.LegacyIncomingMessagesPort), logger, "legacyAgentListener")

	pinger := pingsender.NewPingSender(PING_SENDER_INTERVAL)
	dropsondeMessageListener, dropsondeMessageChan := eventlistener.NewEventListener("localhost:"+strconv.Itoa(config.DropsondeIncomingMessagesPort), logger, "dropsondeAgentListener", pinger)
	dropsondeClientPool, dropsondeServerDiscovery := initializeClientPool(config, logger, config.LoggregatorDropsondePort)

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

	component := InitializeComponent(config, logger, instrumentables)

	go collectorregistrar.NewCollectorRegistrar(cfcomponent.DefaultYagnatsClientProvider, component, time.Duration(config.CollectorRegistrarIntervalMilliseconds)*time.Millisecond, &config.Config).Run()

	go startMonitoringEndpoints(component, logger)
	dropsondeEventChan := make(chan *events.Envelope)

	go legacyMessageListener.Start()

	logEnvelopesChan := make(chan *logmessage.LogEnvelope)
	go legacyUnmarshaller.Run(legacyMessageChan, logEnvelopesChan)
	go legacyMessageConverter.Run(logEnvelopesChan, dropsondeEventChan)

	go dropsondeMessageListener.Start()

	go unmarshaller.Run(dropsondeMessageChan, dropsondeEventChan)

	aggregatedEventChan := make(chan *events.Envelope)
	go messageAggregator.Run(dropsondeEventChan, aggregatedEventChan)

	forwardedEventChan := make(chan *events.Envelope)
	go varzForwarder.Run(aggregatedEventChan, forwardedEventChan)

	reMarshalledMessageChan := make(chan []byte)
	go marshaller.Run(forwardedEventChan, reMarshalledMessageChan)

	signedMessageChan := make(chan ([]byte))
	go signMessages(config.SharedSecret, reMarshalledMessageChan, signedMessageChan)

	go dropsondeServerDiscovery.Run(time.Duration(config.EtcdQueryIntervalMilliseconds) * time.Millisecond)

	forwardMessagesToDoppler(dropsondeClientPool, signedMessageChan, logger)
}

func signMessages(sharedSecret string, dropsondeMessageChan <-chan ([]byte), signedMessageChan chan<- ([]byte)) {
	for message := range dropsondeMessageChan {
		signedMessage := signature.SignMessage(message, []byte(sharedSecret))
		signedMessageChan <- signedMessage
	}
}

func startMonitoringEndpoints(component cfcomponent.Component, logger *gosteno.Logger) {
	if err := component.StartMonitoringEndpoints(); err != nil {
		component.Logger.Error(err.Error())
	}
}

func initializeClientPool(config Config, logger *gosteno.Logger, port int) (*clientpool.LoggregatorClientPool, servicediscovery.ServerAddressList) {
	adapter := StoreAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	err := adapter.Connect()
	if err != nil {
		logger.Errorf("Error connecting to ETCD: %v", err)
	}

	serverAddressDiscovery := servicediscovery.NewServerAddressList(adapter, "/healthstatus/doppler/"+config.Zone, logger)

	clientPool := clientpool.NewLoggregatorClientPool(logger, port, serverAddressDiscovery)
	return clientPool, serverAddressDiscovery
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

func InitializeComponent(config Config, logger *gosteno.Logger, instrumentables []instrumentation.Instrumentable) cfcomponent.Component {
	if len(config.NatsHosts) == 0 {
		logger.Warn("Startup: Did not receive a NATS host - not going to register component")
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSConn, error) {
			return fakeyagnats.Connect(), nil
		}
	}

	component, err := cfcomponent.NewComponent(logger, "MetronAgent", config.Index, &MetronHealthMonitor{}, config.VarzPort, []string{config.VarzUser, config.VarzPass}, instrumentables)
	if err != nil {
		panic(err)
	}

	return component
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
