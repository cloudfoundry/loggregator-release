package dea_logging_agent

import (
	"io/ioutil"
	"runtime"
	"time"
	"fmt"
)

type Config struct {
	instancesJsonFilePath string
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
			json, err := ioutil.ReadFile(config.instancesJsonFilePath)
			if (err != nil) {
				fmt.Printf("ERROR: Reading failed. %v\n", err)
				close(instancesChan)
				return
			}

			time.Sleep(1 * time.Millisecond)
			currentInstances, err := ReadInstances(json)
			if (err != nil) {
				fmt.Printf("ERROR: Failed parsing json %v: %v Trying again...\n", err, string(json))
				runtime.Gosched()
				continue
			}

			for _, instance := range knownInstances {
				_, present := currentInstances[instance.Identifier()]
				if present {
					continue
				}

				delete(knownInstances, instance.Identifier())
				instancesChan <- InstanceEvent{instance, false}
			}

			for _, instance := range currentInstances {
				_, present := knownInstances[instance.Identifier()]
				if present {
					continue
				}

				knownInstances[instance.Identifier()] = instance
				instancesChan <- InstanceEvent{instance, true}
			}
		}
	}()

	return instancesChan
}
