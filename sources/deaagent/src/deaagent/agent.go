package deaagent

import (
	"github.com/cloudfoundry/gosteno"
	"io/ioutil"
	"runtime"
	"time"
	"deaagent/loggregatorclient"
)

type Config struct {
	InstancesJsonFilePath string
	LoggregatorAddress    string
}

const bufferSize = 4096

var logger *gosteno.Logger
var config *Config

func Start(givenConfig *Config, givenLogger *gosteno.Logger) {
	logger = givenLogger
	config = givenConfig

	loggregatorClient := loggregatorclient.NewLoggregatorClient(config.LoggregatorAddress, logger, bufferSize)

	newInstances := watchInstancesJsonFileForChanges()
	for {
		instance := <-newInstances
		logger.Warnf("Starting to listen to %v\n", instance.identifier())
		instance.startListening(loggregatorClient)
	}
}

func watchInstancesJsonFileForChanges() chan *instance {
	knownInstances := make(map[string]bool)
	instancesChan := make(chan *instance)

	go pollInstancesJson(instancesChan, knownInstances)

	return instancesChan
}

func pollInstancesJson(instancesChan chan *instance, knownInstances map[string]bool) {
	for {
		json, err := ioutil.ReadFile(config.InstancesJsonFilePath)
		if err != nil {
			logger.Warnf("Reading failed. %s\n", err)
			close(instancesChan)
			return
		}

		runtime.Gosched()
		time.Sleep(1*time.Millisecond)
		currentInstances, err := readInstances(json)
		if err != nil {
			logger.Warnf("Failed parsing json %s: %v Trying again...\n", err, string(json))
			runtime.Gosched()
			continue
		}

		for instanceIdentifier, _ := range knownInstances {
			_, present := currentInstances[instanceIdentifier]
			if present {
				continue
			}

			delete(knownInstances, instanceIdentifier)
			logger.Infof("Removing stale instance %v", instanceIdentifier)
		}

		for _, instance := range currentInstances {
			_, present := knownInstances[instance.identifier()]
			if present {
				continue
			}

			knownInstances[instance.identifier()] = true
			logger.Infof("Adding new instance %v", instance.identifier())
			instancesChan <- &instance
		}
	}
}
