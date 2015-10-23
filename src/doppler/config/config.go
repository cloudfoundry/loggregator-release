package config

import (
	"doppler/iprange"
	"errors"
	"time"

	"crypto/tls"
	"encoding/json"
	"os"
)

const HeartbeatInterval = 10 * time.Second

type TLSListenerConfig struct {
	Port    uint32
	CrtFile string
	KeyFile string
	Cert    tls.Certificate
}

type Config struct {
	Syslog                        string
	EtcdUrls                      []string
	EtcdMaxConcurrentRequests     int
	Index                         uint
	DropsondeIncomingMessagesPort uint32
	OutgoingPort                  uint32
	LogFilePath                   string
	MaxRetainedLogMessages        uint32
	MessageDrainBufferSize        uint
	SharedSecret                  string
	SkipCertVerify                bool
	BlackListIps                  []iprange.IPRange
	JobName                       string
	Zone                          string
	ContainerMetricTTLSeconds     int
	SinkInactivityTimeoutSeconds  int
	SinkIOTimeoutSeconds          int
	UnmarshallerCount             int
	MetronAddress                 string
	MonitorIntervalSeconds        uint
	SinkDialTimeoutSeconds        int
	EnableTLSTransport            bool
	TLSListenerConfig             TLSListenerConfig
}

func (c *Config) validate() (err error) {
	if c.MaxRetainedLogMessages == 0 {
		return errors.New("Need max number of log messages to retain per application")
	}

	if c.BlackListIps != nil {
		err = iprange.ValidateIpAddresses(c.BlackListIps)
		if err != nil {
			return err
		}
	}

	if c.EnableTLSTransport {
		if c.TLSListenerConfig.CrtFile == "" || c.TLSListenerConfig.KeyFile == "" || c.TLSListenerConfig.Port == 0 {
			return errors.New("invalid TLS listener configuration")
		}
	}

	return err
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

	err = config.validate()
	if err != nil {
		return nil, err
	}

	if config.MonitorIntervalSeconds == 0 {
		config.MonitorIntervalSeconds = 60
	}

	if config.SinkDialTimeoutSeconds == 0 {
		config.SinkDialTimeoutSeconds = 1
	}

	if config.UnmarshallerCount == 0 {
		config.UnmarshallerCount = 1
	}

	if config.EtcdMaxConcurrentRequests < 1 {
		config.EtcdMaxConcurrentRequests = 1
	}

	if config.EnableTLSTransport {
		cert, err := tls.LoadX509KeyPair(config.TLSListenerConfig.CrtFile, config.TLSListenerConfig.KeyFile)
		if err != nil {
			return nil, err
		}
		config.TLSListenerConfig.Cert = cert
	}

	return config, nil
}
