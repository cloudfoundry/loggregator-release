package deaagent

import (
	"deaagent/domain"
	"io/ioutil"
	"path"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/howeyc/fsnotify"
)

type Agent struct {
	InstancesJsonFilePath string
	logger                *gosteno.Logger
	knownInstancesChan    chan<- func(map[string]*TaskListener)
	stopChan              chan struct{}
	sync.WaitGroup
}

func NewAgent(instancesJsonFilePath string, logger *gosteno.Logger) *Agent {
	knownInstancesChan := atomicCacheOperator()

	return &Agent{
		InstancesJsonFilePath: instancesJsonFilePath,
		logger:                logger,
		knownInstancesChan:    knownInstancesChan,
		stopChan:              make(chan struct{}),
	}
}

func (agent *Agent) Start() {
	agent.Add(1)
	go agent.pollInstancesJson()
}

func (agent *Agent) Stop() {
	close(agent.stopChan)
	agent.knownInstancesChan <- resetCache
	agent.Wait()
	close(agent.knownInstancesChan)
}

func (agent *Agent) processTasks(currentTasks map[string]domain.Task) func(knownTasks map[string]*TaskListener) {
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

			agent.logger.Debugf("Adding new task %s", task.Identifier())
			listener, err := NewTaskListener(task, agent.logger)
			if err != nil {
				continue
			}
			knownTasks[identifier] = listener

			go func() {
				agent.Add(1)
				defer func() {
					agent.knownInstancesChan <- removeFromCache(identifier)
					agent.Done()
				}()
				listener.StartListening()
			}()
		}
	}
}

func (agent *Agent) pollInstancesJson() {
	defer agent.Done()

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
	agent.processInstancesJson()

	for {
		select {
		case ev := <-watcher.Event:
			agent.logger.Debugf("Got Event: %v\n", ev)
			if ev.IsDelete() {
				agent.knownInstancesChan <- resetCache
				metrics.SendValue("totalApps", 0.0, "apps")
			} else {
				agent.processInstancesJson()
			}
		case err := <-watcher.Error:
			agent.logger.Warnf("Received error from file system notification: %s\n", err)
		case <-agent.stopChan:
			return
		}
	}
}

func (agent *Agent) processInstancesJson() {
	currentTasks, err := agent.readInstancesJson()
	metrics.SendValue("totalApps", float64(len(currentTasks)), "apps")
	if err != nil {
		return
	}

	agent.knownInstancesChan <- agent.processTasks(currentTasks)
}

func (agent *Agent) readInstancesJson() (map[string]domain.Task, error) {
	json, err := ioutil.ReadFile(agent.InstancesJsonFilePath)
	if err != nil {
		agent.logger.Warnf("Reading failed, retrying. %s\n", err)
		return nil, err
	}

	currentTasks, err := domain.ReadTasks(json)
	if err != nil {
		agent.logger.Errorf("Failed parsing json %s \n", err)
		return nil, err
	}

	return currentTasks, nil
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
