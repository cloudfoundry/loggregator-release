package main

import (
	"dea_logging_agent"
	"flag"
)

var instancesJsonFilePath = flag.String("instancesFile", "/var/vcap/data/dea_next/db/instances.json", "The DEA instances JSON file")
var logFilePath = flag.String("logFile", "/var/vcap/sys/log/dea_logging_agent/dea_logging_agent.log", "The agent log file")
var loggregatorAddress = flag.String("server", "localhost:2345", "The loggregator TCP host:port for log forwarding")

func main() {
	flag.Parse()
	config := &dea_logging_agent.Config{
		InstancesJsonFilePath: *instancesJsonFilePath,
		LogFilePath: *logFilePath,
		LoggregatorAddress: *loggregatorAddress}

	loggregatorClient := &dea_logging_agent.TcpLoggregatorClient{Config: config}

	instanceEvents := dea_logging_agent.WatchInstancesJsonFileForChanges(config)
	for {
		instanceEvent := <-instanceEvents
		if instanceEvent.Addition {
			instanceEvent.Instance.StartListening(loggregatorClient)
		} else {
			instanceEvent.Instance.StopListening()
		}
	}
}
