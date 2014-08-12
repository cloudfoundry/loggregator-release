package deaagent

import (
	"deaagent/domain"
	"github.com/cloudfoundry/dropsonde/emitter/logemitter"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/howeyc/fsnotify"
	"io/ioutil"
	"path"
	"time"
)

type Agent struct {
	InstancesJsonFilePath string
	logger                *gosteno.Logger
	knownInstancesChan    chan<- func(map[string]*TaskListener)
	appStoreUpdateChan    chan<- appservice.AppServices
}

func NewAgent(instancesJsonFilePath string, logger *gosteno.Logger, appStoreUpdateChan chan<- appservice.AppServices) *Agent {
	knownInstancesChan := atomicCacheOperator()

	return &Agent{
		InstancesJsonFilePath: instancesJsonFilePath,
		logger:                logger,
		knownInstancesChan:    knownInstancesChan,
		appStoreUpdateChan:    appStoreUpdateChan,
	}
}

func (agent *Agent) Start(emitter logemitter.Emitter) {
	go agent.pollInstancesJson(emitter)
}

func (agent *Agent) processTasks(currentTasks map[string]domain.Task, emitter logemitter.Emitter) func(knownTasks map[string]*TaskListener) {
	return func(knownTasks map[string]*TaskListener) {
		agent.logger.Debug("Reading tasks data after event on instances.json")
		agent.logger.Debugf("Current known tasks are %v", knownTasks)
		for taskIdentifier, _ := range knownTasks {
			_, present := currentTasks[taskIdentifier]
			if present {
				continue
			}
			knownTasks[taskIdentifier].StopListening()
			delete(knownTasks, taskIdentifier)
			agent.logger.Debugf("Removing stale task %v", taskIdentifier)
		}

		for _, task := range currentTasks {
			identifier := task.Identifier()
			_, present := knownTasks[identifier]

			if present {
				continue
			}

			drainUrls := task.DrainUrls
			if drainUrls == nil {
				drainUrls = []string{}
			}

			task.DrainUrls = nil

			agent.logger.Infof("Updating services for %s to %#v", task.ApplicationId, drainUrls)
			agent.appStoreUpdateChan <- appservice.AppServices{AppId: task.ApplicationId, Urls: drainUrls}

			agent.logger.Debugf("Adding new task %s", task.Identifier())
			listener := NewTaskListener(task, emitter, agent.logger)
			knownTasks[identifier] = listener

			go func() {
				defer func() {
					agent.knownInstancesChan <- removeFromCache(identifier)
				}()
				listener.StartListening()
			}()
		}
	}
}

func (agent *Agent) pollInstancesJson(emitter logemitter.Emitter) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	for {
		time.Sleep(100 * time.Millisecond)
		err := watcher.Watch(path.Dir(agent.InstancesJsonFilePath))
		if err != nil {
			agent.logger.Warnf("Reading failed, retrying. %s\n", err)
			continue
		}
		break
	}

	agent.logger.Info("Read initial tasks data")
	agent.readInstancesJson(emitter)

	for {
		select {
		case ev := <-watcher.Event:
			agent.logger.Debugf("Got Event: %v\n", ev)
			if ev.IsDelete() {
				agent.knownInstancesChan <- resetCache
			} else {
				agent.readInstancesJson(emitter)
			}
		case err := <-watcher.Error:
			agent.logger.Warnf("Received error from file system notification: %s\n", err)
		}
	}
}

func (agent *Agent) readInstancesJson(emitter logemitter.Emitter) {
	json, err := ioutil.ReadFile(agent.InstancesJsonFilePath)
	if err != nil {
		agent.logger.Warnf("Reading failed, retrying. %s\n", err)
		return
	}

	currentTasks, err := domain.ReadTasks(json)
	if err != nil {
		agent.logger.Warnf("Failed parsing json %s: %v Trying again...\n", err, string(json))
		return
	}

	agent.knownInstancesChan <- agent.processTasks(currentTasks, emitter)
}

func removeFromCache(taskId string) func(knownTasks map[string]*TaskListener) {
	return func(knownTasks map[string]*TaskListener) {
		delete(knownTasks, taskId)
	}
}

func resetCache(knownTasks map[string]*TaskListener) {
	for _, task := range knownTasks {
		task.StopListening()
	}
	knownTasks = make(map[string]*TaskListener)
}

func atomicCacheOperator() chan<- func(map[string]*TaskListener) {
	operations := make(chan func(map[string]*TaskListener))
	go func() {
		knownTasks := make(map[string]*TaskListener)
		for operation := range operations {
			operation(knownTasks)
		}
	}()
	return operations
}
