package app

import (
	"log"
	"time"

	envstruct "code.cloudfoundry.org/go-envstruct"
)

// LogAccessAuthorization holds the configuration for verifying log access.
type LogAccessAuthorization struct {
	Addr       string `env:"LOG_ACCESS_ADDR,        required, report"`
	CertPath   string `env:"LOG_ACCESS_CERT_PATH,   required, report"`
	KeyPath    string `env:"LOG_ACCESS_KEY_PATH,    required, report"`
	CAPath     string `env:"LOG_ACCESS_CA_PATH,     required, report"`
	CommonName string `env:"LOG_ACCESS_COMMON_NAME, required, report"`

	ExternalAddr string `env:"LOG_ACCESS_ADDR_EXTERNAL, required, report"`
}

// LogAdminAuthorization holds the configuration for verifing log admin access.
type LogAdminAuthorization struct {
	Addr         string `env:"LOG_ADMIN_ADDR,          required, report"`
	ClientID     string `env:"LOG_ADMIN_CLIENT_ID,     required"`
	ClientSecret string `env:"LOG_ADMIN_CLIENT_SECRET, required"`
	CAPath       string `env:"LOG_ADMIN_CA_PATH,       required, report"`
}

type HTTP struct {
	GatewayAddr string `env:"GATEWAY_ADDR, report"`
	CertPath    string `env:"HTTP_CERT_PATH,   required, report"`
	KeyPath     string `env:"HTTP_KEY_PATH,    required, report"`
}

// Config holds the configuration for the RLP Gateway
type Config struct {
	LogsProviderAddr           string `env:"LOGS_PROVIDER_ADDR,             required, report"`
	LogsProviderCAPath         string `env:"LOGS_PROVIDER_CA_PATH,          required, report"`
	LogsProviderClientCertPath string `env:"LOGS_PROVIDER_CLIENT_CERT_PATH, required, report"`
	LogsProviderClientKeyPath  string `env:"LOGS_PROVIDER_CLIENT_KEY_PATH,  required, report"`
	LogsProviderCommonName     string `env:"LOGS_PROVIDER_COMMON_NAME, report"`
	SkipCertVerify             bool   `env:"SKIP_CERT_VERIFY, report"`

	PProfPort uint32 `env:"PPROF_PORT"`

	LogAccessAuthorization LogAccessAuthorization
	LogAdminAuthorization  LogAdminAuthorization

	StreamTimeout time.Duration `env:"STREAM_TIMEOUT, report"`

	HTTP HTTP
}

// LoadConfig will load and return the config from the current environment. If
// this fails this function will fatally log.
func LoadConfig() Config {
	cfg := Config{
		HTTP: HTTP{
			GatewayAddr: "127.0.0.1:8088",
		},
		LogsProviderCommonName: "reverselogproxy",
		StreamTimeout:          14 * time.Minute,
	}

	if err := envstruct.Load(&cfg); err != nil {
		log.Fatalf("failed to load config from environment: %s", err)
	}

	envstruct.WriteReport(&cfg)

	return cfg
}
