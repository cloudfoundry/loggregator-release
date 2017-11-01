package api

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/net/idna"
)

type GRPC struct {
	Port         uint16
	CAFile       string
	CertFile     string
	KeyFile      string
	CipherSuites []string
}

type Config struct {
	Syslog     string
	Deployment string
	Zone       string
	Job        string
	Index      string

	DisableUDP      bool
	IncomingUDPPort int

	GRPC GRPC

	SharedSecret string // TODO: Delete when UDP is removed

	DopplerAddr       string
	DopplerAddrWithAZ string
	DopplerAddrUDP    string // TODO: Delete when UDP is removed

	MetricBatchIntervalMilliseconds  uint
	RuntimeStatsIntervalMilliseconds uint

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
		MetricBatchIntervalMilliseconds:  5000,
		RuntimeStatsIntervalMilliseconds: 15000,
	}
	err := json.NewDecoder(reader).Decode(config)
	if err != nil {
		return nil, err
	}

	config.DopplerAddrWithAZ, err = idna.ToASCII(config.DopplerAddrWithAZ)
	if err != nil {
		return nil, err
	}
	config.DopplerAddrWithAZ = strings.Replace(config.DopplerAddrWithAZ, "@", "-", -1)

	if config.DopplerAddr == "" {
		return nil, fmt.Errorf("DopplerAddr is required")
	}

	if config.DopplerAddrUDP == "" {
		return nil, fmt.Errorf("DopplerAddrUDP is required")
	}

	return config, nil
}
