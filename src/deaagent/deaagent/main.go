package main

import (
	"deaagent"
	"encoding/json"
	"errors"
	"flag"
	"logger"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/cloudfoundry/dropsonde"
)

const DrainStoreRefreshInterval = 1 * time.Minute

type Config struct {
	Syslog        string
	Index         uint
	MetronAddress string
}

func (c *Config) validate() error {
	if c.MetronAddress == "" {
		return errors.New("Need Metron address (host:port).")
	}

	return nil
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
	config, err := readConfig(*configFile)
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

	log := logger.NewLogger(*logLevel, *logFilePath, "deaagent", config.Syslog)
	log.Info("Startup: Setting up the loggregator dea logging agent")
	// ** END Config Setup

	agent := deaagent.NewAgent(*instancesJsonFilePath, log)

	go agent.Start()

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Interrupt)

	dumpChan := registerGoRoutineDumpSignalChannel()

	for {
		select {
		case <-dumpChan:
			logger.DumpGoRoutine()
		case <-killChan:
			log.Info("Shutting down")
			return
		}
	}
}

func readConfig(configFile string) (*Config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	err = json.NewDecoder(file).Decode(config)
	if err != nil {
		return nil, err
	}

	return config, config.validate()
}

func registerGoRoutineDumpSignalChannel() chan os.Signal {
	threadDumpChan := make(chan os.Signal)
	signal.Notify(threadDumpChan, syscall.SIGUSR1)

	return threadDumpChan
}
