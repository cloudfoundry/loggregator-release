package main

import (
	"deaagent"
	"deaagent/metadataservice"
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/emitter"
)

type Config struct {
	cfcomponent.Config
	Index                  uint
	WardenSyslogPort       uint
	WardenMetadataEndpoint string
	LoggregatorAddress     string
	SharedSecret           string
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.LoggregatorAddress == "" {
		return errors.New("Need Loggregator address (host:port).")
	}
	if !(c.WardenSyslogPort > 0) {
		return errors.New("Need a valid warden syslog port.")
	}
	if c.WardenMetadataEndpoint == "" {
		return errors.New("Need a valid warden metadata endpoint")
	}

	err = c.Validate(logger)
	return
}

var (
	version     = flag.Bool("version", false, "Version info")
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("debug", false, "Debug logging")
	configFile  = flag.String("config", "config/dea_logging_agent.json", "Location of the DEA loggregator agent config json file")
)

const (
	versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
	gitSha        = `TRAVIS_COMMIT`
)

type DeaAgentHealthMonitor struct {
}

func (hm DeaAgentHealthMonitor) Ok() bool {
	return true
}

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("version: %s\ngitSha: %s\nsourceUrl: https://github.com/cloudfoundry/loggregator/tree/%s\n\n",
			versionNumber, gitSha, gitSha)
		return
	}

	// ** Config Setup
	config := &Config{}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "deaagent", config.Config)
	logger.Info("Startup: Setting up the loggregator dea logging agent")

	err = config.validate(logger)
	if err != nil {
		panic(err)
	}
	// ** END Config Setup

	loggregatorEmitter, err := emitter.NewEmitter(config.LoggregatorAddress, "APP", "NA", config.SharedSecret, logger)

	if err != nil {
		panic(err)
	}

	service := metadataservice.NewRestMetaDataService(config.WardenMetadataEndpoint, logger)
	listener := agentlistener.NewAgentListener(fmt.Sprintf("127.0.0.1:%d", config.WardenSyslogPort), logger)
	agent := deaagent.NewAgent(listener, service, loggregatorEmitter, logger)

	cfc, err := cfcomponent.NewComponent(
		logger,
		"LoggregatorDeaAgent",
		config.Index,
		&DeaAgentHealthMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		[]instrumentation.Instrumentable{loggregatorEmitter.LoggregatorClient},
	)

	if err != nil {
		panic(err)
	}

	cr := collectorregistrar.NewCollectorRegistrar(config.MbusClient, logger)
	err = cr.RegisterWithCollector(cfc)
	if err != nil {
		panic(err)
	}

	go func() {
		err := cfc.StartMonitoringEndpoints()
		if err != nil {
			panic(err)
		}
	}()
	go agent.Start()

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		}
	}
}
