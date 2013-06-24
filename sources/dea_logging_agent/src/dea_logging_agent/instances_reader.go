package dea_logging_agent

import (
	"encoding/json"
	"strconv"
	"errors"
)

func ReadInstances(data []byte) (instances map[string]Instance, error error) {
	type instanceJson struct {
		Application_id        string
		Warden_job_id         uint64
		Warden_container_path string
	}

	type instancesJson struct {
		Instances []instanceJson
	}

	var jsonInstances instancesJson

	if (len(data) < 1) {
		error = errors.New("Empty data, can't parse json")
		return
	}

	error = json.Unmarshal(data, &jsonInstances)

	instances = make(map[string]Instance, len(jsonInstances.Instances))
	for _, jsonInstance := range jsonInstances.Instances {
		var wardenJobId string
		if (jsonInstance.Warden_job_id > 0) {
			wardenJobId = strconv.FormatUint(jsonInstance.Warden_job_id, 10)
		}
		instance := Instance{
			ApplicationId: jsonInstance.Application_id,
			WardenContainerPath: jsonInstance.Warden_container_path,
			WardenJobId: wardenJobId }
		instances[instance.Identifier()] = instance
	}

	return
}
