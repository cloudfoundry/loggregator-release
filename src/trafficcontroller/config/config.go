package config

import (
	"encoding/json"
	"errors"
	"os"
)

type EtcdTLSClientConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

type Config struct {
	EtcdUrls                  []string
	EtcdMaxConcurrentRequests int
	EtcdRequireTLS            bool
	EtcdTLSClientConfig       EtcdTLSClientConfig

	JobName                string
	Index                  string
	Syslog                 string
	ApiHost                string
	DopplerPort            uint32
	OutgoingPort           uint32
	OutgoingDropsondePort  uint32
	MetronHost             string
	MetronPort             int
	SystemDomain           string
	SkipCertVerify         bool
	UaaHost                string
	UaaClient              string
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

	if c.MetronHost == "" {
		c.MetronHost = "127.0.0.1"
	}

	if c.MetronPort == 0 {
		c.MetronPort = 3457
	}
}

func (c *Config) validate() error {
	if c.SystemDomain == "" {
		return errors.New("Need system domain in order to create the proxies")
	}

	if c.EtcdRequireTLS {
		if c.EtcdTLSClientConfig.CertFile == "" || c.EtcdTLSClientConfig.KeyFile == "" || c.EtcdTLSClientConfig.CAFile == "" {
			return errors.New("invalid etcd TLS client configuration")
		}
	}

	return nil
}
