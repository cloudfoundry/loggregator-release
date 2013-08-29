package main

import (
	"flag"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"loggregatorrouter"
)

type Config struct {
	Host         string
	Loggregators []string
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

	h := loggregatorrouter.NewRouter(config.Host, config.Loggregators, logger)
	go h.Start(logger)

	for {
		select {
		case <-cfcomponent.RegisterGoRoutineDumpSignalChannel():
			cfcomponent.DumpGoRoutine()
		}
	}
}
