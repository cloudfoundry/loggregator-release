package dea_logging_agent

import (
	"io/ioutil"
	"runtime"
	"time"
)

type Config struct {
	instancesJsonFilePath string
}

type Instance struct {
	ApplicationId string
	WardenJobId uint64
	WardenContainerPath string
}

type InstanceEvent struct {
	Instance
	addition bool
}

func WatchInstancesJsonFileForChanges(config *Config) (chan InstanceEvent) {
	knownInstances := make(map[string]Instance)
	instancesChan := make(chan InstanceEvent)

	go func() {
		for {
			time.Sleep(1 * time.Millisecond)
			file, err := ioutil.ReadFile(config.instancesJsonFilePath)
			if (err != nil) {
				close(instancesChan)
				return
			}
			currentInstances, err := ReadInstances(file)
			if (err != nil) {
				runtime.Gosched()
				continue
			}

			for _, instance := range currentInstances {
				_, present := knownInstances[instance.ApplicationId]
				if present {
					continue
				}
				knownInstances[instance.ApplicationId] = instance
				instancesChan <- InstanceEvent{instance, true}
			}
		}
	}()

	return instancesChan
}
