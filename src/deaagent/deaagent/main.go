package main

import (
	"deaagent"
	"errors"
	"flag"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/store"
	"github.com/cloudfoundry/loggregatorlib/store/cache"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"
)

type Config struct {
	cfcomponent.Config
	Index                     uint
	LoggregatorAddress        string
	SharedSecret              string
	EtcdUrls                  []string
	EtcdMaxConcurrentRequests int
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.LoggregatorAddress == "" {
		return errors.New("Need Metron address (host:port).")
	}

	err = c.Validate(logger)
	return
}

var (
	logFilePath           = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel              = flag.Bool("debug", false, "Debug logging")
	configFile            = flag.String("config", "config/dea_logging_agent.json", "Location of the DEA loggregator agent config json file")
	instancesJsonFilePath = flag.String("instancesFile", "/var/vcap/data/dea_next/db/instances.json", "The DEA instances JSON file")
	cpuprofile            = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile            = flag.String("memprofile", "", "write memory profile to this file")
)

type DeaAgentHealthMonitor struct {
}

func (hm DeaAgentHealthMonitor) Ok() bool {
	return true
}

func main() {
	flag.Parse()

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
			ticker := time.NewTicker(time.Second * 1)
			defer ticker.Stop()
			for {
				<-ticker.C
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

	appStoreUpdateChan := newRunningAppStoreUpdateChannel(config)
	agent := deaagent.NewAgent(*instancesJsonFilePath, logger, appStoreUpdateChan)

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

func newRunningAppStoreUpdateChannel(config *Config) chan<- appservice.AppServices {
	appStoreUpdateChan := make(chan appservice.AppServices, 10)
	workerPool := workerpool.NewWorkerPool(config.EtcdMaxConcurrentRequests)
	storeAdapter := etcdstoreadapter.NewETCDStoreAdapter(config.EtcdUrls, workerPool)
	storeAdapter.Connect()
	appStoreCache := cache.NewAppServiceCache()
	appStoreWatcher, updateChan, removeChan := store.NewAppServiceStoreWatcher(storeAdapter, appStoreCache)
	appStore := store.NewAppServiceStore(storeAdapter, appStoreWatcher)

	go func() {
		for _ = range updateChan {
		}
	}()
	go func() {
		for _ = range removeChan {
		}
	}()
	go appStore.Run(appStoreUpdateChan)

	return appStoreUpdateChan
}
