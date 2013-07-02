package deaagent

import (
	"github.com/cloudfoundry/gosteno"
	"io/ioutil"
	"runtime"
	"time"
	"deaagent/loggregatorclient"
)

type agent struct {
	InstancesJsonFilePath string
	LoggregatorAddress    string
	logger *gosteno.Logger
}

const bufferSize = 4096

func NewAgent(instancesJsonFilePath string, loggregatorAddress string, logger *gosteno.Logger) (*agent) {
	return &agent{instancesJsonFilePath, loggregatorAddress, logger}
}

func (agent *agent) Start() {
	loggregatorClient := loggregatorclient.NewLoggregatorClient(agent.LoggregatorAddress, agent.logger, bufferSize)

	newInstances := agent.watchInstancesJsonFileForChanges()
	for {
		instance := <-newInstances
		agent.logger.Warnf("Starting to listen to %v\n", instance.identifier())
		instance.startListening(loggregatorClient)
	}
}

func (agent *agent) watchInstancesJsonFileForChanges() chan *instance {
	knownInstances := make(map[string]bool)
	instancesChan := make(chan *instance)

	go agent.pollInstancesJson(instancesChan, knownInstances)

	return instancesChan
}

func (agent *agent) pollInstancesJson(instancesChan chan *instance, knownInstances map[string]bool) {
	for {
		json, err := ioutil.ReadFile(agent.InstancesJsonFilePath)
		if err != nil {
			agent.logger.Warnf("Reading failed. %s\n", err)
			close(instancesChan)
			return
		}

		runtime.Gosched()
		time.Sleep(1*time.Millisecond)
		currentInstances, err := readInstances(json, agent.logger)
		if err != nil {
			agent.logger.Warnf("Failed parsing json %s: %v Trying again...\n", err, string(json))
			runtime.Gosched()
			continue
		}

		for instanceIdentifier, _ := range knownInstances {
			_, present := currentInstances[instanceIdentifier]
			if present {
				continue
			}

			delete(knownInstances, instanceIdentifier)
			agent.logger.Infof("Removing stale instance %v", instanceIdentifier)
		}

		for _, instance := range currentInstances {
			_, present := knownInstances[instance.identifier()]
			if present {
				continue
			}

			knownInstances[instance.identifier()] = true
			agent.logger.Infof("Adding new instance %v", instance.identifier())
			instancesChan <- &instance
		}
	}
}
