package deaagent

import (
	"deaagent/domain"
	"deaagent/syslog_drain_store"
	"github.com/cloudfoundry/gosteno"
	"github.com/howeyc/fsnotify"
	"io/ioutil"
	"path"
	"sync"
	"time"
)

type Agent struct {
	InstancesJsonFilePath     string
	logger                    *gosteno.Logger
	knownInstancesChan        chan<- func(map[string]*TaskListener)
	syslogDrainStore          syslog_drain_store.SyslogDrainStore
	appNodeTTLRefreshInterval time.Duration
	drainStoreRefreshInterval time.Duration
	stopChan                  chan struct{}
	sync.WaitGroup
}

func NewAgent(instancesJsonFilePath string, logger *gosteno.Logger, syslogDrainStore syslog_drain_store.SyslogDrainStore,
	appNodeTTLRefreshInterval, drainStoreRefreshInterval time.Duration) *Agent {
	knownInstancesChan := atomicCacheOperator()

	return &Agent{
		InstancesJsonFilePath:     instancesJsonFilePath,
		logger:                    logger,
		knownInstancesChan:        knownInstancesChan,
		syslogDrainStore:          syslogDrainStore,
		appNodeTTLRefreshInterval: appNodeTTLRefreshInterval,
		drainStoreRefreshInterval: drainStoreRefreshInterval,
		stopChan:                  make(chan struct{}),
	}
}

func (agent *Agent) Start() {
	go agent.pollInstancesJson()
	go agent.runAppNodeRefreshLoop()
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
			appId := task.ApplicationId

			drainUrls := task.DrainUrls
			if drainUrls == nil {
				drainUrls = []string{}
			}

			agent.logger.Infof("Updating services for %s to %#v", appId, drainUrls)
			err := agent.syslogDrainStore.UpdateDrains(appId, drainUrls)
			if err != nil {
				agent.logger.Errorf("Drains for app %v could not be updated. Drain Urls: %v: %v", appId, drainUrls, err)
			}

			identifier := task.Identifier()
			_, present := knownTasks[identifier]

			if present {
				continue
			}

			task.DrainUrls = nil

			agent.logger.Debugf("Adding new task %s", task.Identifier())
			listener := NewTaskListener(task, agent.logger)
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
	agent.Add(1)
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

	ticker := time.NewTicker(agent.drainStoreRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case ev := <-watcher.Event:
			agent.logger.Debugf("Got Event: %v\n", ev)
			if ev.IsDelete() {
				agent.knownInstancesChan <- resetCache
			} else {
				agent.processInstancesJson()
			}
		case <-ticker.C:
			agent.processInstancesJson()
		case err := <-watcher.Error:
			agent.logger.Warnf("Received error from file system notification: %s\n", err)
		case <-agent.stopChan:
			return
		}
	}
}

func (agent *Agent) runAppNodeRefreshLoop() {
	agent.Add(1)
	defer agent.Done()

	ticker := time.NewTicker(agent.appNodeTTLRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			agent.knownInstancesChan <- agent.refreshAppNodes
		case <-agent.stopChan:
			return
		}
	}
}

func (agent *Agent) refreshAppNodes(currentTasks map[string]*TaskListener) {
	for _, taskListener := range currentTasks {
		task := taskListener.Task()
		agent.logger.Debugf("refreshing drain store app node for app %v", task.ApplicationId)
		err := agent.syslogDrainStore.RefreshAppNode(task.ApplicationId)
		if err != nil {
			agent.logger.Errorf("Failed to refresh drain TTL for app %v: %v", task.ApplicationId, err)
		}
	}
}

func (agent *Agent) processInstancesJson() {
	currentTasks, err := agent.readInstancesJson()
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
		agent.logger.Warnf("Failed parsing json %s: %v Trying again...\n", err, string(json))
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
