package config
import (
	"os"
	"encoding/json"
)

type Config struct{
	DopplerEndpoint string
	SkipSSLVerify bool

	DropsondePort int

	LoginRequired bool
	UaaURL string
	AdminUser string
	AdminPassword string
}

func Load() *Config {
	configFile, err := os.Open(configPath())
	if err != nil {
		panic(err)
	}

	config := &Config{}
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

func configPath() string {
	path := os.Getenv("CONFIG")
	if path == "" {
		panic("Must set $CONFIG to point to an integration config .json file.")
	}

	return path
}