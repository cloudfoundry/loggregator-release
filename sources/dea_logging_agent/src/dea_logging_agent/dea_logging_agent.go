package dea_logging_agent

import (
	steno "github.com/cloudfoundry/gosteno"
	"io/ioutil"
	"runtime"
	"time"
)

type Config struct {
	InstancesJsonFilePath string
	LogFilePath           string
	LoggregatorAddress    string
}

type InstanceEvent struct {
	Instance
	Addition bool
}

func WatchInstancesJsonFileForChanges(config *Config) chan InstanceEvent {
	knownInstances := make(map[string]Instance)
	instancesChan := make(chan InstanceEvent)

	loggingConfig := &steno.Config{
		Sinks: []steno.Sink{
			steno.NewFileSink(config.LogFilePath)},
		Level:     steno.LOG_INFO,
		Codec:     steno.NewJsonCodec(),
		EnableLOC: true}
	steno.Init(loggingConfig)
	logger := steno.NewLogger("dea_logging_agent")

	go pollInstancesJson(config.InstancesJsonFilePath, instancesChan, knownInstances, logger)

	return instancesChan
}

func pollInstancesJson(InstancesJsonFilePath string, instancesChan chan InstanceEvent, knownInstances map[string]Instance, logger *steno.Logger) {
	for {
		json, err := ioutil.ReadFile(InstancesJsonFilePath)
		if err != nil {
			logger.Warnf("Reading failed. %v\n", err)
			close(instancesChan)
			return
		}

		time.Sleep(1 * time.Millisecond)
		currentInstances, err := ReadInstances(json)
		if err != nil {
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
			logger.Infof("Removing stale instance %v", instance.Identifier())
			instancesChan <- InstanceEvent{instance, false}
		}

		for _, instance := range currentInstances {
			_, present := knownInstances[instance.Identifier()]
			if present {
				continue
			}

			knownInstances[instance.Identifier()] = instance
			logger.Infof("Adding new instance %v", instance.Identifier())
			instancesChan <- InstanceEvent{instance, true}
		}
	}
}
