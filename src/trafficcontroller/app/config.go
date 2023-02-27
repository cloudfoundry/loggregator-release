package app

import (
	"errors"
	"log"
	"time"

	"code.cloudfoundry.org/go-envstruct"
)

// Agent stores configuration for communication to a logging/metric agent.
type Agent struct {
	GRPCAddress string `env:"AGENT_GRPC_ADDRESS"`
}

// GRPC stores TLS configuration for gRPC communcation to router and agent.
type GRPC struct {
	CAFile   string `env:"ROUTER_CA_FILE"`
	CertFile string `env:"ROUTER_CERT_FILE"`
	KeyFile  string `env:"ROUTER_KEY_FILE"`
}

// CCTLSClientConfig stores TLS cofiguration for communication with cloud
// controller.
type CCTLSClientConfig struct {
	CertFile   string `env:"CC_CERT_FILE"`
	KeyFile    string `env:"CC_KEY_FILE"`
	CAFile     string `env:"CC_CA_FILE"`
	ServerName string `env:"CC_SERVER_NAME"`
}

// Config stores all Configuration options for trafficcontroller.
type Config struct {
	UseRFC339             bool          `env:"USE_RFC339"`
	IP                    string        `env:"TRAFFIC_CONTROLLER_IP, report"`
	ApiHost               string        `env:"TRAFFIC_CONTROLLER_API_HOST, report"`
	OutgoingDropsondePort uint32        `env:"TRAFFIC_CONTROLLER_OUTGOING_DROPSONDE_PORT, report"`
	OutgoingCertFile      string        `env:"TRAFFIC_CONTROLLER_OUTGOING_CERT_FILE, report"`
	OutgoingKeyFile       string        `env:"TRAFFIC_CONTROLLER_OUTGOING_KEY_FILE, report"`
	SystemDomain          string        `env:"TRAFFIC_CONTROLLER_SYSTEM_DOMAIN, report"`
	SkipCertVerify        bool          `env:"TRAFFIC_CONTROLLER_SKIP_CERT_VERIFY, report"`
	UaaHost               string        `env:"TRAFFIC_CONTROLLER_UAA_HOST, report"`
	UaaClient             string        `env:"TRAFFIC_CONTROLLER_UAA_CLIENT, report"`
	UaaClientSecret       string        `env:"TRAFFIC_CONTROLLER_UAA_CLIENT_SECRET"`
	UaaCACert             string        `env:"TRAFFIC_CONTROLLER_UAA_CA_CERT, report"`
	SecurityEventLog      string        `env:"TRAFFIC_CONTROLLER_SECURITY_EVENT_LOG, report"`
	PProfPort             uint32        `env:"TRAFFIC_CONTROLLER_PPROF_PORT, report"`
	MetricEmitterInterval time.Duration `env:"TRAFFIC_CONTROLLER_METRIC_EMITTER_INTERVAL, report"`
	DisableAccessControl  bool          `env:"TRAFFIC_CONTROLLER_DISABLE_ACCESS_CONTROL, report"`
	RouterAddrs           []string      `env:"ROUTER_ADDRS, report"`

	CCTLSClientConfig CCTLSClientConfig
	Agent             Agent
	GRPC              GRPC
}

// LoadConfig reads from the environment to create a Config.
func LoadConfig() (*Config, error) {
	config := Config{
		MetricEmitterInterval: time.Minute,
	}

	err := envstruct.Load(&config)
	if err != nil {
		return nil, err
	}

	err = config.validate()
	if err != nil {
		return nil, err
	}

	err = envstruct.WriteReport(&config)
	if err != nil {
		log.Printf("Failed to print a report of the from environment: %s\n", err)
	}

	return &config, nil
}

func (c *Config) validate() error {
	if c.SystemDomain == "" {
		return errors.New("Need system domain in order to create the proxies")
	}

	if c.IP == "" {
		return errors.New("Need IP address for access logging")
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

	if c.UaaClientSecret == "" {
		return errors.New("missing UAA client secret")
	}

	if c.UaaHost != "" && c.UaaCACert == "" {
		return errors.New("missing UAA CA certificate")
	}

	return nil
}
