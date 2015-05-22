package main

import (
	"flag"
	"fmt"
	"metron/eventlistener"
	"metron/heartbeatrequester"
	"metron/legacymessage"
	"metron/messageaggregator"
	"metron/tagger"
	"metron/varzforwarder"
	"time"

	"github.com/cloudfoundry/dropsonde/dropsonde_marshaller"
	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/clientpool"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/loggregatorlib/servicediscovery"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
)

var (
	logFilePath    = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	configFilePath = flag.String("config", "config/metron.json", "Location of the Metron config json file")
	debug          = flag.Bool("debug", false, "Debug logging")
)

var metricTTL = time.Second * 5
var pingSenderInterval = time.Second * 1

var storeAdapterProvider = func(urls []string, concurrentRequests int) storeadapter.StoreAdapter {
	workPool := workpool.NewWorkPool(concurrentRequests)

	return etcdstoreadapter.NewETCDStoreAdapter(urls, workPool)
}

func main() {
	flag.Parse()
	config, logger := parseConfig(*debug, *configFilePath, *logFilePath)

	dopplerClientPool := initializeClientPool(config, logger, config.LoggregatorDropsondePort)

	// TODO: delete next three lines when "legacy" format goes away
	legacyMessageListener, legacyMessageChan := agentlistener.NewAgentListener(fmt.Sprintf("localhost:%d", config.LegacyIncomingMessagesPort), logger, "legacyAgentListener")
	legacyUnmarshaller := legacymessage.NewUnmarshaller(logger)
	legacyMessageConverter := legacymessage.NewConverter(logger)

	pinger := heartbeatrequester.New(pingSenderInterval)
	dropsondeMessageListener, dropsondeMessageChan := eventlistener.New(fmt.Sprintf("localhost:%d", config.DropsondeIncomingMessagesPort), logger, "dropsondeAgentListener", pinger)

	unmarshaller := dropsonde_unmarshaller.NewDropsondeUnmarshaller(logger)
	messageAggregator := messageaggregator.New(logger)
	varzForwarder := varzforwarder.New(config.Job, metricTTL, logger)
	marshaller := dropsonde_marshaller.NewDropsondeMarshaller(logger)
	messageTagger := tagger.New(config.Deployment, config.Job, config.Index)

	instrumentables := []instrumentation.Instrumentable{
		legacyMessageListener,
		dropsondeMessageListener,
		unmarshaller,
		varzForwarder,
		messageAggregator,
		marshaller,
	}

	component := initializeComponent(config, logger, instrumentables)

	go collectorregistrar.NewCollectorRegistrar(cfcomponent.DefaultYagnatsClientProvider, component, time.Duration(config.CollectorRegistrarIntervalMilliseconds)*time.Millisecond, &config.Config).Run()
	go startMonitoringEndpoints(component, logger)

	// Produce channels for connecting processing pipeline stages
	dropsondeEventChan := make(chan *events.Envelope)
	logEnvelopesChan := make(chan *logmessage.LogEnvelope)
	aggregatedEventChan := make(chan *events.Envelope)
	taggedEventChan := make(chan *events.Envelope)
	forwardedEventChan := make(chan *events.Envelope)
	reMarshalledMessageChan := make(chan []byte)
	signedMessageChan := make(chan ([]byte))

	// Listen for legacy-format messages, upconvert, and drop onto dropsondeEventChan
	go legacyMessageListener.Start()
	go legacyUnmarshaller.Run(legacyMessageChan, logEnvelopesChan)
	go legacyMessageConverter.Run(logEnvelopesChan, dropsondeEventChan)

	// Listen for dropsonde messages, and drop onto dropsondeEventChan
	go dropsondeMessageListener.Start()
	go unmarshaller.Run(dropsondeMessageChan, dropsondeEventChan)

	// Start the message processing pipeline
	go messageAggregator.Run(dropsondeEventChan, aggregatedEventChan)
	go messageTagger.Run(aggregatedEventChan, taggedEventChan)
	go varzForwarder.Run(taggedEventChan, forwardedEventChan)
	go marshaller.Run(forwardedEventChan, reMarshalledMessageChan)
	go signMessages(config.SharedSecret, reMarshalledMessageChan, signedMessageChan)
	forwardMessagesToDoppler(dopplerClientPool, signedMessageChan, logger)
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

func initializeClientPool(config metronConfig, logger *gosteno.Logger, port int) *clientpool.LoggregatorClientPool {
	adapter := storeAdapterProvider(config.EtcdUrls, config.EtcdMaxConcurrentRequests)
	err := adapter.Connect()
	if err != nil {
		logger.Errorf("Error connecting to ETCD: %v", err)
	}

	inZoneServerAddressList := servicediscovery.NewServerAddressList(adapter, "/healthstatus/doppler/"+config.Zone, logger)
	allZoneServerAddressList := servicediscovery.NewServerAddressList(adapter, "/healthstatus/doppler/", logger)

	go inZoneServerAddressList.Run(time.Duration(config.EtcdQueryIntervalMilliseconds) * time.Millisecond)
	go allZoneServerAddressList.Run(time.Duration(config.EtcdQueryIntervalMilliseconds) * time.Millisecond)

	clientPool := clientpool.NewLoggregatorClientPool(logger, port, inZoneServerAddressList, allZoneServerAddressList)
	return clientPool
}

type metronConfig struct {
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
	Deployment                    string
}

type metronHealthMonitor struct{}

func (*metronHealthMonitor) Ok() bool {
	return true
}

func initializeComponent(config metronConfig, logger *gosteno.Logger, instrumentables []instrumentation.Instrumentable) cfcomponent.Component {
	if len(config.NatsHosts) == 0 {
		logger.Warn("Startup: Did not receive a NATS host - not going to register component")
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSConn, error) {
			return fakeyagnats.Connect(), nil
		}
	}

	component, err := cfcomponent.NewComponent(logger, "MetronAgent", config.Index, &metronHealthMonitor{}, config.VarzPort, []string{config.VarzUser, config.VarzPass}, instrumentables, config.Job)
	if err != nil {
		panic(err)
	}

	return component
}

func parseConfig(debug bool, configFile, logFilePath string) (metronConfig, *gosteno.Logger) {
	config := metronConfig{}
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
