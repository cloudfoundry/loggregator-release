package config

import (
	"encoding/json"
	"errors"
	"io"
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
	OutgoingDropsondePort  uint32
	MetronHost             string
	MetronPort             int
	GRPCPort               int
	SystemDomain           string
	SkipCertVerify         bool
	UaaHost                string
	UaaClient              string
	UaaClientSecret        string
	MonitorIntervalSeconds uint
	SecurityEventLog       string
	PPROFPort              uint32
}

func ParseConfig(configFile string) (*Config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Parse(file)
}

func Parse(r io.Reader) (*Config, error) {
	config := &Config{}

	err := json.NewDecoder(r).Decode(config)
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

	if c.MetronHost == "" {
		c.MetronHost = "127.0.0.1"
	}

	if c.MetronPort == 0 {
		c.MetronPort = 3457
	}

	if c.GRPCPort == 0 {
		c.GRPCPort = 8082
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
