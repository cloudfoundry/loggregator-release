package config

import (
	"encoding/json"
	"os"
)

type TestConfig struct {
	DopplerEndpoint string
	SkipSSLVerify   bool

	DropsondePort int

	EtcdUrls     []string
	SharedSecret string
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
}

func Load() *TestConfig {
	configFile, err := os.Open(configPath())
	if err != nil {
		panic(err)
	}

	config := &TestConfig{}
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(config)
	if err != nil {
		panic(err)
	}

	if config.DropsondePort == 0 {
		config.DropsondePort = 3457
	}

	return config
}

func (tc *TestConfig) SaveMetronConfig() {
	baseMetronConfigFile, err := os.Open("fixtures/bosh_lite_metron.json")
	if err != nil {
		panic(err)
	}

	var metronConfig MetronConfig
	decoder := json.NewDecoder(baseMetronConfigFile)
	err = decoder.Decode(&metronConfig)
	if err != nil {
		panic(err)
	}

	metronConfig.IncomingUDPPort = tc.DropsondePort
	if len(tc.EtcdUrls) != 0 {
		metronConfig.EtcdUrls = tc.EtcdUrls
	}

	if tc.SharedSecret != "" {
		metronConfig.SharedSecret = tc.SharedSecret
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
