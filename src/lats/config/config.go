package config

import (
	"encoding/json"
	"log"
	"os"
)

type TestConfig struct {
	IP              string
	DopplerEndpoint string
	SkipSSLVerify   bool

	DropsondePort int

	EtcdUrls              []string
	SharedSecret          string
	EtcdRequireTLS        bool
	EtcdTLSClientConfig   TLSClientConfig
	MetronTLSClientConfig TLSClientConfig

	ReverseLogProxyAddr string
}

type TLSClientConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

type MetronConfig struct {
	IncomingUDPPort               int
	SharedSecret                  string
	EtcdUrls                      []string
	LoggregatorDropsondePort      int
	Index                         string
	EtcdMaxConcurrentRequests     int
	EtcdQueryIntervalMilliseconds int
	Zone                          string
	EtcdRequireTLS                bool
	EtcdTLSClientConfig           TLSClientConfig
}

func Load() *TestConfig {
	configFile, err := os.Open(configPath())
	if err != nil {
		panic(err)
	}

	config := &TestConfig{
		MetronTLSClientConfig: TLSClientConfig{
			CAFile:   "/var/vcap/jobs/metron_agent/config/certs/loggregator_ca.crt",
			CertFile: "/var/vcap/jobs/metron_agent/config/certs/metron_agent.crt",
			KeyFile:  "/var/vcap/jobs/metron_agent/config/certs/metron_agent.key",
		},
		ReverseLogProxyAddr: "reverse-log-proxy.service.cf.internal:8082",
	}
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(config)
	if err != nil {
		panic(err)
	}

	if config.DropsondePort == 0 {
		config.DropsondePort = 3457
	}

	if config.IP == "" {
		log.Panic("Config requires IP but is missing")
	}

	return config
}

func (tc *TestConfig) SaveMetronConfig() {
	// TODO: Consider removing these default values and forcing user to
	// provide all values. These were initially added as a fixture file to get
	// bosh-lite lats passing. When we converted over to binary blobs the
	// fixture file had to go.
	metronConfig := MetronConfig{
		IncomingUDPPort:          3457,
		SharedSecret:             "PLACEHOLDER-LOGGREGATOR-SECRET",
		EtcdUrls:                 []string{"http://10.244.0.42:4001"},
		LoggregatorDropsondePort: 3457,
		Index: "0",
		EtcdMaxConcurrentRequests: 1,
		Zone: "z1",
	}

	metronConfig.IncomingUDPPort = tc.DropsondePort
	if len(tc.EtcdUrls) != 0 {
		metronConfig.EtcdUrls = tc.EtcdUrls
	}

	if tc.SharedSecret != "" {
		metronConfig.SharedSecret = tc.SharedSecret
	}

	if tc.EtcdRequireTLS {
		metronConfig.EtcdRequireTLS = true
		metronConfig.EtcdTLSClientConfig = tc.EtcdTLSClientConfig
	}

	metronConfigFile, err := os.Create("fixtures/metron.json")
	bytes, err := json.Marshal(metronConfig)
	if err != nil {
		panic(err)
	}

	metronConfigFile.Write(bytes)
	metronConfigFile.Close()
}

func configPath() string {
	path := os.Getenv("CONFIG")
	if path == "" {
		panic("Must set $CONFIG to point to an integration config .json file.")
	}

	return path
}
