package config

import (
	"doppler/iprange"
	"errors"
	"time"

	"encoding/json"
	"os"
)

const HeartbeatInterval = 10 * time.Second

type TLSListenerConfig struct {
	Port     uint32
	CertFile string
	KeyFile  string
	CAFile   string
}

type Config struct {
	BlackListIps                    []iprange.IPRange
	ContainerMetricTTLSeconds       int
	IncomingUDPPort                 uint32
	IncomingTCPPort                 uint32
	EnableTLSTransport              bool
	TLSListenerConfig               TLSListenerConfig
	EtcdMaxConcurrentRequests       int
	EtcdUrls                        []string
	Index                           string
	JobName                         string
	LogFilePath                     string
	MaxRetainedLogMessages          uint32
	MessageDrainBufferSize          uint
	MetricBatchIntervalMilliseconds uint
	MetronAddress                   string
	MonitorIntervalSeconds          uint
	OutgoingPort                    uint32
	SharedSecret                    string
	SinkDialTimeoutSeconds          int
	SinkIOTimeoutSeconds            int
	SinkInactivityTimeoutSeconds    int
	SinkSkipCertVerify              bool
	Syslog                          string
	UnmarshallerCount               int
	WebsocketWriteTimeoutSeconds    int
	Zone                            string
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
		if c.TLSListenerConfig.CertFile == "" || c.TLSListenerConfig.KeyFile == "" || c.TLSListenerConfig.Port == 0 {
			return errors.New("invalid TLS listener configuration")
		}
	}

	return err
}

func ParseConfig(configFile string) (*Config, error) {
	config := &Config{
		IncomingUDPPort: 3456,
		IncomingTCPPort: 3457,
	}

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

	// TODO: These probably belong in the Config literal, above.
	// However, in the interests of not breaking things, we're
	// leaving them for further team discussion.
	if config.MetricBatchIntervalMilliseconds == 0 {
		config.MetricBatchIntervalMilliseconds = 5000
	}

	if config.MonitorIntervalSeconds == 0 {
		config.MonitorIntervalSeconds = 60
	}

	if config.SinkDialTimeoutSeconds == 0 {
		config.SinkDialTimeoutSeconds = 1
	}

	if config.WebsocketWriteTimeoutSeconds == 0 {
		config.WebsocketWriteTimeoutSeconds = 30
	}

	if config.UnmarshallerCount == 0 {
		config.UnmarshallerCount = 1
	}

	if config.EtcdMaxConcurrentRequests < 1 {
		config.EtcdMaxConcurrentRequests = 1
	}

	return config, nil
}
