package main

import (
	"flag"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
)

var (
	logFilePath    = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	configFilePath = flag.String("configFile", "config/metron.json", "Location of the Metron config json file")
	debug          = flag.Bool("debug", false, "Debug logging")
)

func main() {
	flag.Parse()

	config, logger := parseConfig(*debug, *configFilePath, *logFilePath)

	component := InitializeComponent(DefaultRegistrarFactory, config, logger)

	if err := component.StartMonitoringEndpoints(); err != nil {
		component.Logger.Error(err.Error())
	}
}

type Config struct {
	cfcomponent.Config
	Index uint
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

func InitializeComponent(registrarFactory RegistrarFactory, config Config, logger *gosteno.Logger) *cfcomponent.Component {
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

	component, err := cfcomponent.NewComponent(logger, "MetronAgent", config.Index, &MetronHealthMonitor{}, config.VarzPort, []string{config.VarzUser, config.VarzPass}, []instrumentation.Instrumentable{})
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
