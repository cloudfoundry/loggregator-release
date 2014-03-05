package main

import (
	"deaagent"
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"
)

type Config struct {
	cfcomponent.Config
	Index              uint
	LoggregatorAddress string
	SharedSecret       string
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.LoggregatorAddress == "" {
		return errors.New("Need Loggregator address (host:port).")
	}

	err = c.Validate(logger)
	return
}

var (
	version               = flag.Bool("version", false, "Version info")
	logFilePath           = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel              = flag.Bool("debug", false, "Debug logging")
	configFile            = flag.String("config", "config/dea_logging_agent.json", "Location of the DEA loggregator agent config json file")
	instancesJsonFilePath = flag.String("instancesFile", "/var/vcap/data/dea_next/db/instances.json", "The DEA instances JSON file")
	cpuprofile            = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile            = flag.String("memprofile", "", "write memory profile to this file")
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

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer func() {
			pprof.StopCPUProfile()
			f.Close()
		}()
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			panic(err)
		}
		go func() {
			defer f.Close()
			for {
				<-time.After(time.Second * 1)
				pprof.WriteHeapProfile(f)
			}
		}()
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

	agent := deaagent.NewAgent(*instancesJsonFilePath, logger)

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
	go agent.Start(loggregatorEmitter)

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Kill, os.Interrupt)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		case <-killChan:
			logger.Info("Shutting down")
			return
		}
	}
}
