package dea_logging_agent

import (
	"encoding/json"
)

func ReadInstances(data []byte) ([]Instance, error) {
	type instanceJson struct {
		Application_id string
	}

	type instancesJson struct {
		Instances []instanceJson
	}

	var instances []Instance
	var jsonInstances instancesJson
	error := json.Unmarshal(data, &jsonInstances)

	instances = make([]Instance, len(jsonInstances.Instances))
	for i, jsonInstance := range jsonInstances.Instances {
		instances[i] = Instance{ApplicationId: jsonInstance.Application_id}
	}

	return instances, error
}
