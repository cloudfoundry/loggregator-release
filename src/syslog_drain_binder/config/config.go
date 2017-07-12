package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type EtcdTLSClientConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

type MutualTLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

type Config struct {
	DisableSyslogDrains bool

	InstanceName          string
	DrainUrlTtlSeconds    int64
	UpdateIntervalSeconds int64

	EtcdMaxConcurrentRequests int
	EtcdUrls                  []string
	EtcdRequireTLS            bool
	EtcdTLSClientConfig       EtcdTLSClientConfig

	MetronAddress string

	CloudControllerAddress   string
	CloudControllerTLSConfig MutualTLSConfig
	PollingBatchSize         int

	SkipCertVerify bool
	PPROFPort      uint32
}

func ParseConfig(configFile string) (*Config, error) {
	config := Config{}

	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		return nil, err
	}

	err = config.validate()
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (config Config) validate() error {
	if config.MetronAddress == "" {
		return errors.New("Need Metron address (host:port)")
	}

	if config.EtcdMaxConcurrentRequests < 1 {
		return fmt.Errorf("Need EtcdMaxConcurrentRequests ≥ 1, received %d", config.EtcdMaxConcurrentRequests)
	}

	if config.EtcdRequireTLS {
		if config.EtcdTLSClientConfig.CertFile == "" ||
			config.EtcdTLSClientConfig.KeyFile == "" ||
			config.EtcdTLSClientConfig.CAFile == "" {
			return errors.New("Invalid etcd TLS client configuration")
		}
	}

	return nil
}
