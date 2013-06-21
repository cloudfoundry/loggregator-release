package dea_logging_agent

import (
	"io/ioutil"
	"encoding/json"
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
	instances := make(chan Instance)

	go func() {
		file, err := ioutil.ReadFile(config.instancesJsonFilePath)
		if (err != nil) {
			close(instances)
			return
		}
		var currentInstances Instances
		err = json.Unmarshal(file, &currentInstances)
		if (err != nil) {
			panic(err)
		}
		for _, instance := range currentInstances.Instances {
			instances <- instance
		}
	}()

	return instances
}
