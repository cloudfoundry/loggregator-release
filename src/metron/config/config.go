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

type Protocols map[Protocol]struct{}

func (p *Protocols) UnmarshalJSON(value []byte) error {
	var prots []Protocol
	if err := json.Unmarshal(value, &prots); err != nil {
		return err
	}
	set := make(Protocols)
	for _, prot := range prots {
		set[prot] = struct{}{}
	}
	*p = set
	return nil
}

func (p Protocols) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Strings())
}

func (p Protocols) Strings() []string {
	protocols := make([]string, 0, len(p))
	for protocol := range p {
		protocols = append(protocols, string(protocol))
	}
	return protocols
}

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

	SharedSecret string

	MetricBatchIntervalMilliseconds  uint
	RuntimeStatsIntervalMilliseconds uint

	TCPBatchSizeBytes            uint64
	TCPBatchIntervalMilliseconds uint

	Protocols Protocols
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
		Protocols:                        Protocols{"udp": struct{}{}},
	}
	err := json.NewDecoder(reader).Decode(config)
	if err != nil {
		return nil, err
	}
	if len(config.Protocols) == 0 {
		return nil, errors.New("Metron cannot start without protocols")
	}
	if config.EtcdRequireTLS {
		if config.EtcdTLSClientConfig.CertFile == "" || config.EtcdTLSClientConfig.KeyFile == "" || config.EtcdTLSClientConfig.CAFile == "" {
			return nil, errors.New("invalid etcd TLS client configuration")
		}
	}

	return config, nil
}
