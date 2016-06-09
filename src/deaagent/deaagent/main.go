package main

import (
	"deaagent"
	"encoding/json"
	"errors"
	"flag"
	"logger"
	"os"
	"time"

	"signalmanager"

	"github.com/cloudfoundry/dropsonde"
)

const DrainStoreRefreshInterval = 1 * time.Minute

type Config struct {
	Syslog        string
	Index         string
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

	log := logger.NewLogger(*logLevel, *logFilePath, "deaagent", config.Syslog)
	log.Info("Startup: Setting up the loggregator dea logging agent")
	// ** END Config Setup

	agent := deaagent.NewAgent(*instancesJsonFilePath, log)

	go agent.Start()

	killChan := signalmanager.RegisterKillSignalChannel()
	dumpChan := signalmanager.RegisterGoRoutineDumpSignalChannel()

	for {
		select {
		case <-dumpChan:
			signalmanager.DumpGoRoutine()
		case <-killChan:
			log.Info("Shutting down")
			os.Exit(0)
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
