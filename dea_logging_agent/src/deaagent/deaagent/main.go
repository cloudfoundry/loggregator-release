package main

import (
	"deaagent"
	"errors"
	"flag"
	"fmt"
	cfmessagebus "github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
)

type Config struct {
	Index              uint
	VarzPort           uint32
	VarzUser           string
	VarzPass           string
	NatsHost           string
	NatsPort           int
	NatsUser           string
	NatsPass           string
	LoggregatorAddress string
	mbusClient         cfmessagebus.MessageBus
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.VarzPass == "" || c.VarzUser == "" || c.VarzPort == 0 {
		return errors.New("Need VARZ username/password/port.")
	}

	if c.LoggregatorAddress == "" {
		return errors.New("Need Loggregator address (host:port).")
	}

	c.mbusClient, err = cfmessagebus.NewMessageBus("NATS")
	if err != nil {
		return errors.New(fmt.Sprintf("Can not create message bus to NATS: %s", err))
	}
	c.mbusClient.Configure(c.NatsHost, c.NatsPort, c.NatsUser, c.NatsPass)
	c.mbusClient.SetLogger(logger)
	err = c.mbusClient.Connect()
	if err != nil {
		return errors.New(fmt.Sprintf("Could not connect to NATS: %v", err.Error()))
	}

	return nil
}

var version = flag.Bool("version", false, "Version info")
var logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
var logLevel = flag.Bool("v", false, "Verbose logging")
var configFile = flag.String("config", "config/dea_logging_agent.json", "Location of the DEA loggregator agent config json file")
var instancesJsonFilePath = flag.String("instancesFile", "/var/vcap/data/dea_next/db/instances.json", "The DEA instances JSON file")

const versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
const gitSha = `TRAVIS_COMMIT`

type DeaAgentHealthMonitor struct {
}

func (hm DeaAgentHealthMonitor) Ok() bool {
	return true
}

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("\n\nversion: %s\ngitSha: %s\n\n", versionNumber, gitSha)
		return
	}

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "deaagent")

	// ** Config Setup
	config := &Config{}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}

	err = config.validate(logger)
	if err != nil {
		panic(err)
	}

	// ** END Config Setup

	loggregatorClient := loggregatorclient.NewLoggregatorClient(config.LoggregatorAddress, logger, 4096)

	agent := deaagent.NewAgent(*instancesJsonFilePath, logger)

	cfc, err := cfcomponent.NewComponent(
		"",
		0,
		"LoggregatorDeaAgent",
		config.Index,
		&DeaAgentHealthMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		[]instrumentation.Instrumentable{loggregatorClient},
	)

	if err != nil {
		panic(err)
	}

	cr := collectorregistrar.NewCollectorRegistrar(config.mbusClient, logger)
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
	go agent.Start(loggregatorClient)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		}
	}
}
