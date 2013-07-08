package deaagent

import (
	"encoding/json"
	"errors"
	"github.com/cloudfoundry/gosteno"
)

func readInstances(data []byte, logger *gosteno.Logger) (instances map[string]instance, err error) {
	type instanceJson struct {
		Application_id        string
		Warden_job_id         uint64
		Warden_container_path string
		Instance_index        uint64
		State                 string
	}

	type instancesJson struct {
		Instances []instanceJson
	}

	var jsonInstances instancesJson

	if len(data) < 1 {
		err = errors.New("Empty data, can't parse json")
		return
	}

	err = json.Unmarshal(data, &jsonInstances)

	instances = make(map[string]instance, len(jsonInstances.Instances))
	for _, jsonInstance := range jsonInstances.Instances {
		if jsonInstance.State == "RUNNING" {
			instance := instance{
				applicationId:       jsonInstance.Application_id,
				wardenContainerPath: jsonInstance.Warden_container_path,
				wardenJobId:         jsonInstance.Warden_job_id,
				index:               jsonInstance.Instance_index,
				logger:              logger}
			instances[instance.identifier()] = instance
		}
	}

	return
}
