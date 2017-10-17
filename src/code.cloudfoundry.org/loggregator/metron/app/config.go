package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

type GRPC struct {
	Port         uint16
	CAFile       string
	CertFile     string
	KeyFile      string
	CipherSuites []string
}

type Config struct {
	Deployment string
	Zone       string
	Job        string
	Index      string
	IP         string

	Tags map[string]string

	DisableUDP         bool
	IncomingUDPPort    int
	HealthEndpointPort uint

	GRPC GRPC

	DopplerAddr       string
	DopplerAddrWithAZ string

	MetricBatchIntervalMilliseconds  uint
	RuntimeStatsIntervalMilliseconds uint

	// TODO: These should be removed once we better understand what the
	// value should be set to.
	BatchInterval int
	BatchSize     int

	PPROFPort uint32
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
		MetricBatchIntervalMilliseconds:  60000,
		RuntimeStatsIntervalMilliseconds: 60000,
		BatchInterval:                    int(500 * time.Millisecond),
		BatchSize:                        100,
	}
	err := json.NewDecoder(reader).Decode(config)
	if err != nil {
		return nil, err
	}

	if config.DopplerAddr == "" {
		return nil, fmt.Errorf("DopplerAddr is required")
	}

	if config.DopplerAddrWithAZ == "" {
		return nil, fmt.Errorf("DopplerAddrWithAZ is required")
	}

	return config, nil
}
