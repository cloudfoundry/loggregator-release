package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
	"loggregatorrouter"
)

type Config struct {
	cfcomponent.Config
	Host         string
	Loggregators []string
}

func (c *Config) validate(logger *gosteno.Logger) (err error) {
	if len(c.Loggregators) < 1 || c.Loggregators[0] == "" {
		return errors.New("Need a loggregator server (host:port).")
	}

	err = c.Validate(logger)

	return
}

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("debug", false, "Debug logging")
	version     = flag.Bool("version", false, "Version info")
	configFile  = flag.String("config", "config/loggregator_router.json", "Location of the loggregator router config json file")
)

const versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
const gitSha = `TRAVIS_COMMIT`

func main() {
	flag.Parse()

	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "udprouter")

	if *version {
		fmt.Printf("\n\nversion: %s\ngitSha: %s\n\n", versionNumber, gitSha)
		return
	}
	config := &Config{Host: "0.0.0.0:3456"}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}
	err = config.validate(logger)
	if err != nil {
		panic(err)
	}

	h := loggregatorrouter.NewHasher(config.Loggregators)
	r, err := loggregatorrouter.NewRouter(config.Host, h, config.Config, logger)
	if err != nil {
		panic(err)
	}

	cr := collectorregistrar.NewCollectorRegistrar(config.MbusClient, logger)
	err = cr.RegisterWithCollector(r.Component)
	if err != nil {
		panic(err)
	}

	go func() {
		err := r.StartMonitoringEndpoints()
		if err != nil {
			panic(err)
		}
	}()

	go r.Start(logger)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		}
	}
}
