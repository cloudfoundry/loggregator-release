package main

import (
	"errors"
	"flag"
	"fmt"
	cfmessagebus "github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"loggregatorrouter"
)

type Config struct {
	Host         string
	Loggregators []string
	VarzPort     uint32
	VarzUser     string
	VarzPass     string
	NatsHost     string
	NatsPort     int
	NatsUser     string
	NatsPass     string
	mbusClient   cfmessagebus.MessageBus
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

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("debug", false, "Debug logging")
	version     = flag.Bool("version", false, "Version info")
	configFile  = flag.String("config", "config/loggregator_router.json", "Location of the loggregator router config json file")
)

const versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
const gitSha = `TRAVIS_COMMIT`

type LoggregatorRouterMonitor struct {
}

func (hm LoggregatorRouterMonitor) Ok() bool {
	return true
}

func main() {
	flag.Parse()

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "udprouter")

	if *version {
		fmt.Printf("\n\nversion: %s\ngitSha: %s\n\n", versionNumber, gitSha)
		return
	}
	config := &Config{Host: "0.0.0.0:3456"}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}
	err = config.validate(logger)
	if err != nil {
		panic(err)
	}

	h := loggregatorrouter.NewRouter(config.Host, config.Loggregators, logger)
	cfc, err := cfcomponent.NewComponent(
		"",
		0,
		"LoggregatorDeaAgent",
		config.Index,
		&LoggregatorRouterMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		[]instrumentation.Instrumentable{h},
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

	go h.Start(logger)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		}
	}
}
