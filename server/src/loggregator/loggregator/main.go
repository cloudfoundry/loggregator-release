package main

import (
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"loggregator"
	"os"
	"strings"
)

var version = flag.Bool("version", false, "Version info")
var sourceHost = flag.String("source", "0.0.0.0:3456", "ip:port to listen for source messages")
var apiHost = flag.String("apiHost", "http://0.0.0.0:9876", "protocol:ip:port to communicate with Cloud Foundry API server")
var webHost = flag.String("web", "0.0.0.0:8080", "ip:port to listen for web requests")
var logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
var logLevel = flag.Bool("v", false, "Verbose logging")

const versionNumber = `0.0.1.TRAVIS_BUILD_NUMBER`
const gitSha = `TRAVIS_COMMIT`

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("\n\nversion: %s\ngitSha: %s\n\n", versionNumber, gitSha)
		return
	}

	level := gosteno.LOG_INFO

	if *logLevel {
		level = gosteno.LOG_DEBUG
	}

	loggingConfig := &gosteno.Config{
		Sinks:     make([]gosteno.Sink, 1),
		Level:     level,
		Codec:     gosteno.NewJsonCodec(),
		EnableLOC: true}

	if strings.TrimSpace(*logFilePath) == "" {
		loggingConfig.Sinks[0] = gosteno.NewIOSink(os.Stdout)
	} else {
		loggingConfig.Sinks[0] = gosteno.NewFileSink(*logFilePath)
	}
	gosteno.Init(loggingConfig)
	logger := gosteno.NewLogger("loggregator")

	listener := loggregator.NewAgentListener(*sourceHost, logger)
	incomingData := listener.Start()

	cfSinkServer := loggregator.NewCfSinkServer(incomingData, logger, *webHost, "/tail/", *apiHost)
	cfSinkServer.Start()
}
