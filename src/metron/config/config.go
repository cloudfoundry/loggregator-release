package config

import (
	"encoding/json"
	"errors"
	"io"
	"os"
)

const (
	kilobyte               = 1024
	defaultBatchSize       = 10 * kilobyte
	defaultBatchIntervalMS = 100
)

type EtcdTLSClientConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

type TLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

type GRPC struct {
	Port     int
	CAFile   string
	CertFile string
	KeyFile  string
}

type Config struct {
	Syslog     string
	Deployment string
	Zone       string
	Job        string
	Index      string

	IncomingUDPPort int

	EtcdUrls                      []string
	EtcdMaxConcurrentRequests     int
	EtcdQueryIntervalMilliseconds int
	EtcdRequireTLS                bool
	EtcdTLSClientConfig           EtcdTLSClientConfig

	GRPC GRPC

	SharedSecret string

	MetricBatchIntervalMilliseconds  uint
	RuntimeStatsIntervalMilliseconds uint

	TCPBatchSizeBytes            uint64
	TCPBatchIntervalMilliseconds uint

	TLSConfig TLSConfig
	PPROFPort uint32

	// DEPRECATED
	LoggregatorDropsondePort int
	PreferredProtocol        string
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
	config := &Config{
		TCPBatchSizeBytes:                defaultBatchSize,
		TCPBatchIntervalMilliseconds:     defaultBatchIntervalMS,
		MetricBatchIntervalMilliseconds:  5000,
		RuntimeStatsIntervalMilliseconds: 15000,
	}
	err := json.NewDecoder(reader).Decode(config)
	if err != nil {
		return nil, err
	}

	if config.EtcdRequireTLS {
		if config.EtcdTLSClientConfig.CertFile == "" || config.EtcdTLSClientConfig.KeyFile == "" || config.EtcdTLSClientConfig.CAFile == "" {
			return nil, errors.New("invalid etcd TLS client configuration")
		}
	}

	return config, nil
}
