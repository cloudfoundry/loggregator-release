package dea_logging_agent

import (
	"io/ioutil"
	"runtime"
	"time"
	steno "github.com/cloudfoundry/gosteno"
)

type Config struct {
	instancesJsonFilePath string
	logFilePath           string
}

type InstanceEvent struct {
	Instance
	addition bool
}

func WatchInstancesJsonFileForChanges(config *Config) (chan InstanceEvent) {
	knownInstances := make(map[string]Instance)
	instancesChan := make(chan InstanceEvent)

	loggingConfig := &steno.Config{
		Sinks: []steno.Sink{
			steno.NewFileSink(config.logFilePath)},
		Level:     steno.LOG_INFO,
		Codec:     steno.NewJsonCodec(),
		EnableLOC: true }
	steno.Init(loggingConfig)
	logger := steno.NewLogger("dea_logging_agent")

	go pollInstancesJson(config.instancesJsonFilePath, instancesChan, knownInstances, logger)

	return instancesChan
}

func pollInstancesJson(instancesJsonFilePath string, instancesChan chan InstanceEvent, knownInstances map[string]Instance, logger *steno.Logger) {
	for {
		json, err := ioutil.ReadFile(instancesJsonFilePath)
		if (err != nil) {
			logger.Warnf("Reading failed. %v\n", err)
			close(instancesChan)
			return
		}

		time.Sleep(1*time.Millisecond)
		currentInstances, err := ReadInstances(json)
		if (err != nil) {
			logger.Warnf("Failed parsing json %v: %v Trying again...\n", err, string(json))
			runtime.Gosched()
			continue
		}

		for _, instance := range knownInstances {
			_, present := currentInstances[instance.Identifier()]
			if present {
				continue
			}

			delete(knownInstances, instance.Identifier())
			instancesChan <- InstanceEvent{instance, false}
		}

		for _, instance := range currentInstances {
			_, present := knownInstances[instance.Identifier()]
			if present {
				continue
			}

			knownInstances[instance.Identifier()] = instance
			instancesChan <- InstanceEvent{instance, true}
		}
	}
}
