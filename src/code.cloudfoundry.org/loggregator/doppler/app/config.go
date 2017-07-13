package app

import (
	"errors"
	"io/ioutil"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/internal/iprange"

	"encoding/json"
	"os"
)

const HeartbeatInterval = 10 * time.Second

type MetronConfig struct {
	UDPAddress  string
	GRPCAddress string
}

type EtcdTLSClientConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

type GRPC struct {
	Port         uint16
	CAFile       string
	CertFile     string
	KeyFile      string
	CipherSuites []string
}

type Config struct {
	DisableSyslogDrains             bool
	DisableAnnounce                 bool
	BlackListIps                    []iprange.IPRange
	ContainerMetricTTLSeconds       int
	IncomingUDPPort                 uint32
	EtcdMaxConcurrentRequests       int
	EtcdUrls                        []string
	EtcdRequireTLS                  bool
	EtcdTLSClientConfig             EtcdTLSClientConfig
	Index                           string
	JobName                         string
	IP                              string
	MaxRetainedLogMessages          uint32
	MessageDrainBufferSize          uint
	MetricBatchIntervalMilliseconds uint
	MetronConfig                    MetronConfig
	MonitorIntervalSeconds          uint
	WebsocketHost                   string
	OutgoingPort                    uint32
	GRPC                            GRPC
	SharedSecret                    string
	SinkDialTimeoutSeconds          int
	SinkIOTimeoutSeconds            int
	SinkInactivityTimeoutSeconds    int
	SinkSkipCertVerify              bool
	UnmarshallerCount               int
	WebsocketWriteTimeoutSeconds    int
	Zone                            string
	PPROFPort                       uint32
	HealthAddr                      string
}

func (c *Config) validate() (err error) {
	if c.IP == "" {
		return errors.New("invalid doppler config, no IP provided")
	}

	if c.MaxRetainedLogMessages == 0 {
		return errors.New("Need max number of log messages to retain per application")
	}

	if c.BlackListIps != nil {
		err = iprange.ValidateIpAddresses(c.BlackListIps)
		if err != nil {
			return err
		}
	}

	if c.EtcdRequireTLS {
		if c.EtcdTLSClientConfig.CertFile == "" || c.EtcdTLSClientConfig.KeyFile == "" || c.EtcdTLSClientConfig.CAFile == "" {
			return errors.New("invalid etcd TLS client configuration")
		}
	}

	if len(c.GRPC.CAFile) == 0 {
		return errors.New("invalid doppler config, no GRPC.CAFile provided")
	}

	if len(c.GRPC.CertFile) == 0 {
		return errors.New("invalid doppler config, no GRPC.CertFile provided")
	}

	if len(c.GRPC.KeyFile) == 0 {
		return errors.New("invalid doppler config, no GRPC.KeyFile provided")
	}

	return nil
}

func ParseConfig(configFile string) (*Config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return Parse(b)
}

func Parse(confData []byte) (*Config, error) {
	config := &Config{
		IncomingUDPPort: 3456,
	}

	err := json.Unmarshal(confData, config)
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

	if config.GRPC.Port == 0 {
		config.GRPC.Port = 8082
	}

	if config.HealthAddr == "" {
		config.HealthAddr = "localhost:14825"
	}

	return config, nil
}
