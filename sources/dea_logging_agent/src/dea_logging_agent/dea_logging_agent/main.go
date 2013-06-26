package main

import (
	steno "github.com/cloudfoundry/gosteno"
	"dea_logging_agent"
	"flag"
	"os"
	"strings"
)

var instancesJsonFilePath = flag.String("instancesFile", "/var/vcap/data/dea_next/db/instances.json", "The DEA instances JSON file")
var logFilePath = flag.String("logFile", "", "The agent log file, defaults to STDOUT")
var loggregatorAddress = flag.String("server", "localhost:2345", "The loggregator TCP host:port for log forwarding")

func main() {
	flag.Parse()

	loggingConfig := &steno.Config{
		Sinks: make([]steno.Sink, 1),
		Level:     steno.LOG_INFO,
		Codec:     steno.NewJsonCodec(),
		EnableLOC: true}
	if strings.TrimSpace(*logFilePath) == "" {
		loggingConfig.Sinks[0] = steno.NewIOSink(os.Stdout)
	} else {
		loggingConfig.Sinks[0] = steno.NewFileSink(*logFilePath)
	}
	steno.Init(loggingConfig)
	logger := steno.NewLogger("dea_logging_agent")

	config := &dea_logging_agent.Config{
		InstancesJsonFilePath: *instancesJsonFilePath,
		LoggregatorAddress: *loggregatorAddress}

	deaLoggingAgent := &dea_logging_agent.Agent{Config:config}
	deaLoggingAgent.Start(logger)
}
