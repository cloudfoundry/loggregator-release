package config

import (
	"errors"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
)

type Config struct {
	EtcdUrls                  []string
	EtcdMaxConcurrentRequests int

	JobName  string
	JobIndex int
	Zone     string
	cfcomponent.Config
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
}

func ParseConfig(logLevel *bool, configFile, logFilePath *string) (*Config, *gosteno.Logger, error) {
	config := &Config{OutgoingPort: 8080}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		return nil, nil, err
	}

	config.setDefaults()
	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "loggregator trafficcontroller", config.Config)
	logger.Info("Startup: Setting up the loggregator traffic controller")

	err = config.validate(logger)
	if err != nil {
		return nil, nil, err
	}
	return config, logger, nil
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
}

func (c *Config) validate(logger *gosteno.Logger) error {
	if c.SystemDomain == "" {
		return errors.New("Need system domain in order to create the proxies")
	}
	return nil
}
