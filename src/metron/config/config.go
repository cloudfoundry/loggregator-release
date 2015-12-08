package config

import (
	"encoding/json"
	"io"
	"os"
)

type TLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

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

	MetricBatchIntervalMilliseconds  uint
	RuntimeStatsIntervalMilliseconds uint

	PreferredProtocol string
	TLSConfig         TLSConfig
	BufferSize        int
	EnableBuffer      bool
}

func ParseConfig(configFile string) (*Config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Parse(file)
}

func Parse(reader io.Reader) (*Config, error) {
	config := &Config{}
	err := json.NewDecoder(reader).Decode(config)
	if err != nil {
		return nil, err
	}

	if config.MetricBatchIntervalMilliseconds == 0 {
		config.MetricBatchIntervalMilliseconds = 5000
	}

	if config.RuntimeStatsIntervalMilliseconds == 0 {
		config.RuntimeStatsIntervalMilliseconds = 15000
	}

	if config.PreferredProtocol == "" {
		config.PreferredProtocol = "udp"
	}

	if config.BufferSize < 3 {
		config.BufferSize = 100
	}

	return config, nil
}
