package config

import (
	"encoding/json"
	"errors"
	"os"
)

type Config struct {
	EtcdUrls                  []string
	EtcdMaxConcurrentRequests int

	JobName                string
	JobIndex               int
	Zone                   string
	Syslog                 string
	ApiHost                string
	DopplerPort            uint32
	OutgoingPort           uint32
	OutgoingDropsondePort  uint32
	MetronPort             int
	SystemDomain           string
	SkipCertVerify         bool
	UaaHost                string
	UaaClientId            string
	UaaClientSecret        string
	MonitorIntervalSeconds uint
	SecurityEventLog       string
}

func ParseConfig(logLevel bool, configFile string) (*Config, error) {
	config := &Config{}

	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(config)
	if err != nil {
		return nil, err
	}

	config.setDefaults()

	err = config.validate()
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (c *Config) setDefaults() {
	if c.JobName == "" {
		c.JobName = "loggregator_trafficcontroller"
	}

	if c.EtcdMaxConcurrentRequests < 1 {
		c.EtcdMaxConcurrentRequests = 10
	}

	if c.MonitorIntervalSeconds == 0 {
		c.MonitorIntervalSeconds = 60
	}

	if c.OutgoingPort == 0 {
		c.OutgoingPort = 8080
	}
}

func (c *Config) validate() error {
	if c.SystemDomain == "" {
		return errors.New("Need system domain in order to create the proxies")
	}
	return nil
}
