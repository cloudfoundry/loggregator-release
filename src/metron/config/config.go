package config

import (
	"bytes"
	"encoding/json"
	"errors"
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
	case "udp", "tls", "tcp":
		*p = Protocol(value)
	default:
		return fmt.Errorf("Invalid protocol: %s", valueStr)
	}
	return nil
}

type Protocols []Protocol

func (p Protocols) Strings() []string {
	protocols := make([]string, 0, len(p))
	for _, protocol := range p {
		protocols = append(protocols, string(protocol))
	}
	return protocols
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

	Protocols Protocols
	TLSConfig TLSConfig

	// DEPRECATED
	PreferredProtocol string
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
		Protocols:                        []Protocol{"udp"},
	}
	err := json.NewDecoder(reader).Decode(config)
	if err != nil {
		return nil, err
	}
	if len(config.Protocols) == 0 {
		return nil, errors.New("Metron cannot start without protocols")
	}

	return config, nil
}
