package main

import (
	"flag"
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"strings"
	"os"
	"loggregator"
)

var version = flag.Bool("version", false, "Version info")
var host = flag.String("host", ":3456", "server ip:port to listen")
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

	listener := loggregator.NewAgentListener(*host, logger)
	incomingData := listener.Start()

	cfSink := loggregator.NewCfSink(incomingData, logger)
	cfSink.Start()
}
