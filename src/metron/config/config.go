package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Syslog     string
	Deployment string
	Zone       string
	Job        string
	Index      uint

	DropsondeIncomingMessagesPort int

	EtcdUrls                      []string
	EtcdMaxConcurrentRequests     int
	EtcdQueryIntervalMilliseconds int

	LoggregatorDropsondePort int
	SharedSecret             string

	MetricBatchIntervalSeconds uint
}

func ParseConfig(configFile string) (*Config, error) {
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

	if config.MetricBatchIntervalSeconds == 0 {
		config.MetricBatchIntervalSeconds = 15
	}

	return config, nil
}
