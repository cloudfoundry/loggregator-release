package dea_logging_agent

import (
	steno "github.com/cloudfoundry/gosteno"
	"io/ioutil"
	"runtime"
	"time"
)

type Config struct {
	InstancesJsonFilePath string
	LoggregatorAddress    string
}

type InstanceEvent struct {
	Instance
	Addition bool
}

var logger *steno.Logger
var config *Config

func Start(givenConfig *Config, givenLogger *steno.Logger) {
	loggregatorClient := &TcpLoggregatorClient{Config: config}
	logger = givenLogger
	config = givenConfig

	instanceEvents := WatchInstancesJsonFileForChanges()
	for {
		instanceEvent := <-instanceEvents
		if instanceEvent.Addition {
			logger.Warnf("Starting to listen to %v\n", instanceEvent.Identifier())
			instanceEvent.Instance.StartListening(loggregatorClient)
		} else {
			logger.Warnf("Stopping listening to %v\n", instanceEvent.Identifier())
			instanceEvent.Instance.StopListening()
		}
	}
}

func WatchInstancesJsonFileForChanges() chan InstanceEvent {
	knownInstances := make(map[string]Instance)
	instancesChan := make(chan InstanceEvent)

	go pollInstancesJson(instancesChan, knownInstances)

	return instancesChan
}

func pollInstancesJson(instancesChan chan InstanceEvent, knownInstances map[string]Instance) {
	for {
		json, err := ioutil.ReadFile(config.InstancesJsonFilePath)
		if err != nil {
			logger.Warnf("Reading failed. %v\n", err)
			close(instancesChan)
			return
		}

		time.Sleep(1*time.Millisecond)
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
