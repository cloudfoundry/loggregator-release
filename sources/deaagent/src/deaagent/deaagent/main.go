package main

import (
	"github.com/cloudfoundry/gosteno"
	"deaagent"
	"flag"
	"os"
	"strings"
	"fmt"
)

var instancesJsonFilePath = flag.String("instancesFile", "/var/vcap/data/dea_next/db/instances.json", "The DEA instances JSON file")
var logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
var loggregatorAddress = flag.String("server", "localhost:3456", "The loggregator TCP host:port for log forwarding")
var logLevel = flag.Bool("v", false, "Verbose logging")
var version = flag.Bool("version", false, "Version info")

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
		Sinks: make([]gosteno.Sink, 1),
		Level:     level,
		Codec:     gosteno.NewJsonCodec(),
		EnableLOC: true}
	if strings.TrimSpace(*logFilePath) == "" {
		loggingConfig.Sinks[0] = gosteno.NewIOSink(os.Stdout)
	} else {
		loggingConfig.Sinks[0] = gosteno.NewFileSink(*logFilePath)
	}
	gosteno.Init(loggingConfig)
	logger := gosteno.NewLogger("deaagent")

	config := &deaagent.Config{
		InstancesJsonFilePath: *instancesJsonFilePath,
		LoggregatorAddress: *loggregatorAddress}

	deaagent.Start(config, logger)
}
