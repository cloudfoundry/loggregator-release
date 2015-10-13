package config

import "github.com/cloudfoundry/loggregatorlib/cfcomponent"

type Config struct {
	cfcomponent.Config

	Deployment string
	Zone       string
	Job        string
	Index      uint

	LegacyIncomingMessagesPort    int
	DropsondeIncomingMessagesPort int

	EtcdUrls                      []string
	EtcdMaxConcurrentRequests     int
	EtcdQueryIntervalMilliseconds int

	LoggregatorDropsondePort int
	SharedSecret             string

	MetricBatchIntervalSeconds uint
}

func ParseConfig(configFile *string) (*Config, error) {
	config := &Config{}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		return config, err
	}

	if config.MetricBatchIntervalSeconds == 0 {
		config.MetricBatchIntervalSeconds = 15
	}

	return config, nil
}
