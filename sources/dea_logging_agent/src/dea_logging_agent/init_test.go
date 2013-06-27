package dea_logging_agent

import (
	steno "github.com/cloudfoundry/gosteno"
	"os"
)

func init() {
	config = &Config{
		InstancesJsonFilePath: "/tmp/config.json",
		LoggregatorAddress: "localhost:9876"}
	os.Remove(config.InstancesJsonFilePath)
	logger = steno.NewLogger("foobar")
}

