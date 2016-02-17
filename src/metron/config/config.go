package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const (
	kilobyte               = 1024
	defaultBatchSize       = 10 * kilobyte
	defaultBatchIntervalMS = 100
)

type Protocol string

func (p *Protocol) UnmarshalJSON(value []byte) error {
	value = bytes.Trim(value, `"`)
	valueStr := string(value)
	switch valueStr {
	case "udp", "tls":
		*p = Protocol(value)
	default:
		return fmt.Errorf("Invalid protocol: %s", valueStr)
	}
	return nil
}

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

	IncomingUDPPort int

	EtcdUrls                      []string
	EtcdMaxConcurrentRequests     int
	EtcdQueryIntervalMilliseconds int

	LoggregatorDropsondePort int
	SharedSecret             string

	MetricBatchIntervalMilliseconds  uint
	RuntimeStatsIntervalMilliseconds uint

	TCPBatchSizeBytes            uint64
	TCPBatchIntervalMilliseconds uint

	PreferredProtocol Protocol
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
	config := &Config{
		TCPBatchSizeBytes:                defaultBatchSize,
		TCPBatchIntervalMilliseconds:     defaultBatchIntervalMS,
		MetricBatchIntervalMilliseconds:  5000,
		RuntimeStatsIntervalMilliseconds: 15000,
		PreferredProtocol:                "udp",
	}
	err := json.NewDecoder(reader).Decode(config)
	if err != nil {
		return nil, err
	}

	if config.BufferSize < 3 {
		config.BufferSize = 100
	}

	return config, nil
}
