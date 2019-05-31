package app

import (
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

// GRPC stores the configuration for the RLP as a server using a PORT with
// mTLS certs and as a client also using mTSL certs for emitting metrics and
// for connecting to the Router.
type GRPC struct {
	Port         int      `env:"RLP_PORT"`
	CertFile     string   `env:"RLP_CERT_FILE"`
	KeyFile      string   `env:"RLP_KEY_FILE"`
	CAFile       string   `env:"RLP_CA_FILE"`
	CipherSuites []string `env:"RLP_CIPHER_SUITES"`
}

// Config stores all configurations options for RLP.
type Config struct {
	PProfPort             uint32        `env:"RLP_PPROF_PORT"`
	HealthAddr            string        `env:"RLP_HEALTH_ADDR"`
	MetricEmitterInterval time.Duration `env:"RLP_METRIC_EMITTER_INTERVAL"`
	MetricSourceID        string        `env:"RLP_METRIC_SOURCE_ID"`
	RouterAddrs           []string      `env:"ROUTER_ADDRS, required"`
	AgentAddr             string        `env:"AGENT_ADDR"`
	MaxEgressStreams      int64         `env:"MAX_EGRESS_STREAMS"`
	GRPC                  GRPC
}

// LoadConfig reads from the environment to create a Config.
func LoadConfig() (*Config, error) {
	conf := Config{
		PProfPort:             6061,
		HealthAddr:            "localhost:14825",
		MetricEmitterInterval: time.Minute,
		MetricSourceID:        "reverse_log_proxy",
		AgentAddr:             "localhost:3458",
		MaxEgressStreams:      500,
	}

	err := envstruct.Load(&conf)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}
