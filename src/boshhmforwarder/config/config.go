package config

import (
	"encoding/json"
	"io/ioutil"

	"boshhmforwarder/logging"
	"errors"
)

type Config struct {
	IncomingPort int
	InfoPort     int
	MetronPort   int
	LogLevel     logging.LogLevel
	DebugPort    int
	Syslog       string
}

func Configuration(configFilePath string) *Config {
	if configFilePath == "" {
		logging.Log.Panic("Missing configuration file path.", nil)
	}

	configFileBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		logging.Log.Panic("Error loading file: ", err)
	}

	config := &Config{
		LogLevel: logging.INFO,
	}

	if err := json.Unmarshal(configFileBytes, config); err != nil {
		logging.Log.Panic("Error unmarshalling configuration", err)
	}

	if err = config.validateProperties(); err != nil {
		logging.Log.Panic("Error validating configuration", err)
	}

	return config
}

func (c *Config) validateProperties() error {
	if c.MetronPort == 0 {
		return errors.New("Metron Port is a required property for the Bosh-HM-Forwarder")
	}

	return nil
}
