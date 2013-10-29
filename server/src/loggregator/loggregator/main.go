package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/agentlistener"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"loggregator/sinkserver"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"time"
)

type Config struct {
	cfcomponent.Config
	Index                  uint
	IncomingPort           uint32
	OutgoingPort           uint32
	LogFilePath            string
	MaxRetainedLogMessages uint
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.MaxRetainedLogMessages == 0 {
		return errors.New("Need max number of log messages to retain per application")
	}

	err = c.Validate(logger)
	return
}

var (
	version     = flag.Bool("version", false, "Version info")
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("debug", false, "Debug logging")
	configFile  = flag.String("config", "config/loggregator.json", "Location of the loggregator config json file")
)

const (
	versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
	gitSha        = `TRAVIS_COMMIT`
)

type LoggregatorServerHealthMonitor struct {
}

func (hm LoggregatorServerHealthMonitor) Ok() bool {
	return true
}

func main() {
	seed := time.Now().UnixNano()
	rand.Seed(seed)

	flag.Parse()

	if *version {
		fmt.Printf("version: %s\ngitSha: %s\nsourceUrl: https://github.com/cloudfoundry/loggregator/tree/%s\n\n",
			versionNumber, gitSha, gitSha)
		return
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "loggregator")

	config := &Config{IncomingPort: 3456, OutgoingPort: 8080}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}
	err = config.validate(logger)
	if err != nil {
		panic(err)
	}

	listener := agentlistener.NewAgentListener(fmt.Sprintf("0.0.0.0:%d", config.IncomingPort), logger)
	incomingData := listener.Start()

	messageRouter := sinkserver.NewMessageRouter(config.MaxRetainedLogMessages, logger)
	httpServer := sinkserver.NewHttpServer(messageRouter, 30*time.Second, logger)

	cfc, err := cfcomponent.NewComponent(
		logger,
		"LoggregatorServer",
		config.Index,
		&LoggregatorServerHealthMonitor{},
		config.VarzPort,
		[]string{config.VarzUser, config.VarzPass},
		[]instrumentation.Instrumentable{listener, messageRouter},
	)

	if err != nil {
		panic(err)
	}

	cr := collectorregistrar.NewCollectorRegistrar(config.MbusClient, logger)
	err = cr.RegisterWithCollector(cfc)
	if err != nil {
		logger.Warnf("Unable to register with collector. Err: %v.", err)
	}

	go func() {
		err := cfc.StartMonitoringEndpoints()
		if err != nil {
			panic(err)
		}
	}()

	go messageRouter.Start()
	go httpServer.Start(incomingData, fmt.Sprintf("0.0.0.0:%d", config.OutgoingPort))

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			break
		}
	}
}
