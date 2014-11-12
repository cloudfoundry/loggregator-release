package main

import (
	"deaagent"
	"deaagent/syslog_drain_store"
	"errors"
	"flag"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"
)

const AppNodeTTLRefreshInterval = syslog_drain_store.APP_NODE_TTL / 2
const DrainStoreRefreshInterval = 1 * time.Minute

type Config struct {
	cfcomponent.Config
	Index                     uint
	MetronAddress             string
	SharedSecret              string
	EtcdUrls                  []string
	EtcdMaxConcurrentRequests int
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if c.MetronAddress == "" {
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

	dropsonde.Initialize(config.MetronAddress, "dea_logging_agent")

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

	if len(config.NatsHosts) == 0 {
		logger.Warn("Startup: Did not receive a NATS host - not going to register component")
		cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSConn, error) {
			return fakeyagnats.Connect(), nil
		}
	}

	err = config.validate(logger)
	if err != nil {
		panic(err)
	}
	// ** END Config Setup

	syslogDrainStore := newSyslogDrainStore(config, logger)
	agent := deaagent.NewAgent(*instancesJsonFilePath, logger, syslogDrainStore, AppNodeTTLRefreshInterval, DrainStoreRefreshInterval)

	go agent.Start()

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

func newSyslogDrainStore(config *Config, logger *gosteno.Logger) syslog_drain_store.SyslogDrainStore {
	workerPool := workerpool.NewWorkerPool(config.EtcdMaxConcurrentRequests)
	storeAdapter := etcdstoreadapter.NewETCDStoreAdapter(config.EtcdUrls, workerPool)
	storeAdapter.Connect()
	return syslog_drain_store.NewSyslogDrainStore(storeAdapter, logger)
}
