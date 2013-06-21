package dea_logging_agent

import (
	"io/ioutil"
	"encoding/json"
	"runtime"
	"time"
)

type Config struct {
	instancesJsonFilePath string
}

type Instance struct {
	AppId string
}

type Instances struct {
	Instances []Instance
}

func WatchInstancesJsonFileForChanges(config *Config) (chan Instance) {
	knownInstances := make(map[string]Instance)
	instancesChan := make(chan Instance)

	go func() {
		for {
			time.Sleep(1 * time.Millisecond)
			file, err := ioutil.ReadFile(config.instancesJsonFilePath)
			if (err != nil) {
				close(instancesChan)
				return
			}
			var currentInstances Instances
			err = json.Unmarshal(file, &currentInstances)
			if (err != nil) {
				runtime.Gosched()
				continue
			}

			for _, instance := range currentInstances.Instances {
				_, present := knownInstances[instance.AppId]
				if present {
					continue
				}
				knownInstances[instance.AppId] = instance
				instancesChan <- instance
			}
		}
	}()

	return instancesChan
}
