package main

import (
	"flag"
	"fmt"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"sourcehasher"
)

type Config struct {
	Host         string
	Loggregators []string
}

var (
	logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
	logLevel    = flag.Bool("v", false, "Verbose logging")
	version     = flag.Bool("version", false, "Version info")
	configFile  = flag.String("config", "config/loggregator_router.json", "Location of the loggregator router config json file")
)

const versionNumber = `0.0.TRAVIS_BUILD_NUMBER`
const gitSha = `TRAVIS_COMMIT`

func main() {
	logger := cfcomponent.NewLogger(*logLevel, *logFilePath, "udprouter")
	flag.Parse()

	if *version {
		fmt.Printf("\n\nversion: %s\ngitSha: %s\n\n", versionNumber, gitSha)
		return
	}
	config := &Config{Host: "10.10.16.14:3456", Loggregators: []string{"10.10.15.14:3456"}}
	err := cfcomponent.ReadConfigInto(config, *configFile)
	if err != nil {
		panic(err)
	}

	h := sourcehasher.NewRouter(config.Host, config.Loggregators, logger)
	go h.Start(logger)
}
