package main

import (
	"flag"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"strconv"
)

var (
	logFilePath    = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	configFilePath = flag.String("configFile", "config/metron.json", "Location of the Metron config json file")
	debug          = flag.Bool("debug", false, "Debug logging")
)

func drainMessages(messageChan <-chan []byte) {
	for _ = range messageChan {
	}
}

func main() {
	flag.Parse()

	config, logger := parseConfig(*debug, *configFilePath, *logFilePath)

	agentListener, messageChan := agentlistener.NewAgentListener("localhost:"+strconv.Itoa(config.UdpListeningPort), logger)

	instrumentables := []instrumentation.Instrumentable{agentListener}
	component := InitializeComponent(DefaultRegistrarFactory, config, logger, instrumentables)

	go agentListener.Start()

	go func() {
		if err := component.StartMonitoringEndpoints(); err != nil {
			component.Logger.Error(err.Error())
		}
	}()

	drainMessages(messageChan)
}

type Config struct {
	cfcomponent.Config
	Index            uint
	UdpListeningPort int
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
