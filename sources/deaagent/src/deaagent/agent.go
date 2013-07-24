package deaagent

import (
	"deaagent/loggregatorclient"
	"github.com/cloudfoundry/gosteno"
	"github.com/howeyc/fsnotify"
	"io/ioutil"
	"path"
	"runtime"
	"time"
)

type agent struct {
	InstancesJsonFilePath string
	logger                *gosteno.Logger
}

const bufferSize = 4096

func NewAgent(instancesJsonFilePath string, logger *gosteno.Logger) *agent {
	return &agent{instancesJsonFilePath, logger}
}

func (agent *agent) Start(loggregatorClient loggregatorclient.LoggregatorClient) {
	newInstances := agent.watchInstancesJsonFileForChanges()
	for {
		instance := <-newInstances
		agent.logger.Infof("Starting to listen to %v\n", instance.identifier())
		instance.startListening(loggregatorClient, agent.logger)
	}
}

func (agent *agent) watchInstancesJsonFileForChanges() chan instance {
	instancesChan := make(chan instance)
	knownInstances := make(map[string]bool)

	readInstancesJson := func() {
		json, err := ioutil.ReadFile(agent.InstancesJsonFilePath)
		if err != nil {
			agent.logger.Warnf("Reading failed, retrying. %s\n", err)
			return
		}

		currentInstances, err := readInstances(json)
		if err != nil {
			agent.logger.Warnf("Failed parsing json %s: %v Trying again...\n", err, string(json))
			return
		}

		agent.logger.Debug("Reading instances data after event on instances.json")
		agent.logger.Debugf("Current known instances are %v", knownInstances)

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
			agent.logger.Infof("Adding new instance %s", instance.identifier())
			instancesChan <- instance
		}
	}

	pollInstancesJson := func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			panic(err)
		}

		for {
			runtime.Gosched()
			time.Sleep(100 * time.Millisecond)
			err := watcher.Watch(path.Dir(agent.InstancesJsonFilePath))
			if err != nil {
				agent.logger.Warnf("Reading failed, retrying. %s\n", err)
				continue
			}
			break
		}

		readInstancesJson()
		agent.logger.Info("Read initial instances data")

		for {
			select {
			case ev := <-watcher.Event:
				agent.logger.Debugf("Got Event: %v\n", ev)
				if ev.IsDelete() {
					knownInstances = make(map[string]bool)
				} else {
					readInstancesJson()
				}
			case err := <-watcher.Error:
				agent.logger.Warnf("Received error from file system notification: %s\n", err)
			}

		}
	}

	go pollInstancesJson()
	return instancesChan
}
