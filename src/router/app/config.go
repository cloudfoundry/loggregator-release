package app

import (
	"errors"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

// Agent stores the configuration for connecting to the Agent over gRPC.
type Agent struct {
	GRPCAddress string `env:"AGENT_GRPC_ADDRESS"`
}

// GRPC stores the configuration for the router as a server using a PORT
// with mTLS certs and as a client also using mTSL certs for emitting metrics.
type GRPC struct {
	Port         uint16   `env:"ROUTER_PORT"`
	CertFile     string   `env:"ROUTER_CERT_FILE"`
	KeyFile      string   `env:"ROUTER_KEY_FILE"`
	CAFile       string   `env:"ROUTER_CA_FILE"`
	CipherSuites []string `env:"ROUTER_CIPHER_SUITES"`
}

// Config stores all configurations options for the Router.
type Config struct {
	GRPC GRPC

	// persistence
	MaxRetainedLogMessages       uint32 `env:"ROUTER_MAX_RETAINED_LOG_MESSAGES"`
	SinkInactivityTimeoutSeconds int    `env:"ROUTER_SINK_INACTIVITY_TIMEOUT_SECONDS"`

	// health
	PProfPort                       uint32 `env:"ROUTER_PPROF_PORT"`
	HealthAddr                      string `env:"ROUTER_HEALTH_ADDR"`
	Agent                           Agent
	MetricBatchIntervalMilliseconds uint   `env:"ROUTER_METRIC_BATCH_INTERVAL_MILLISECONDS"`
	MetricSourceID                  string `env:"ROUTER_METRIC_SOURCE_ID"`
}

// LoadConfig reads from the environment to create a Config.
func LoadConfig() (*Config, error) {
	config := Config{
		MetricBatchIntervalMilliseconds: 5000,
		HealthAddr:                      "localhost:14825",
		MetricSourceID:                  "doppler",
	}

	err := envstruct.Load(&config)
	if err != nil {
		return nil, err
	}

	err = config.validate()
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) validate() (err error) {
	if c.MaxRetainedLogMessages == 0 {
		return errors.New("Need max number of log messages to retain per application")
	}

	if len(c.GRPC.CAFile) == 0 {
		return errors.New("invalid router config, no GRPC.CAFile provided")
	}

	if len(c.GRPC.CertFile) == 0 {
		return errors.New("invalid router config, no GRPC.CertFile provided")
	}

	if len(c.GRPC.KeyFile) == 0 {
		return errors.New("invalid router config, no GRPC.KeyFile provided")
	}

	return nil
}
