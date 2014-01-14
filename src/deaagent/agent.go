package deaagent

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/emitter"
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

func NewAgent(instancesJsonFilePath string, logger *gosteno.Logger) *agent {
	return &agent{instancesJsonFilePath, logger}
}

func (agent *agent) Start(emitter emitter.Emitter) {
	newTasks := agent.watchInstancesJsonFileForChanges()
	for task := range newTasks {
		agent.logger.Infof("Starting to listen to %v\n", task.identifier())
		task.startListening(emitter, agent.logger)
	}
}

func (agent *agent) watchInstancesJsonFileForChanges() chan task {
	tasksChan := make(chan task)
	knownTasks := make(map[string]bool)

	readInstancesJson := func() {
		json, err := ioutil.ReadFile(agent.InstancesJsonFilePath)
		if err != nil {
			agent.logger.Warnf("Reading failed, retrying. %s\n", err)
			return
		}

		currentTasks, err := readTasks(json)
		if err != nil {
			agent.logger.Warnf("Failed parsing json %s: %v Trying again...\n", err, string(json))
			return
		}

		agent.logger.Debug("Reading tasks data after event on instances.json")
		agent.logger.Debugf("Current known tasks are %v", knownTasks)

		for taskIdentifier, _ := range knownTasks {
			_, present := currentTasks[taskIdentifier]
			if present {
				continue
			}

			delete(knownTasks, taskIdentifier)
			agent.logger.Infof("Removing stale task %v", taskIdentifier)
		}

		for _, task := range currentTasks {
			_, present := knownTasks[task.identifier()]
			if present {
				continue
			}

			knownTasks[task.identifier()] = true
			agent.logger.Infof("Adding new task %s", task.identifier())
			tasksChan <- task
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
		agent.logger.Info("Read initial tasks data")

		for {
			select {
			case ev := <-watcher.Event:
				agent.logger.Debugf("Got Event: %v\n", ev)
				if ev.IsDelete() {
					knownTasks = make(map[string]bool)
				} else {
					readInstancesJson()
				}
			case err := <-watcher.Error:
				agent.logger.Warnf("Received error from file system notification: %s\n", err)
			}

		}
	}

	go pollInstancesJson()
	return tasksChan
}
