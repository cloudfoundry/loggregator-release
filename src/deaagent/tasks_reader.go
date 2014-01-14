package deaagent

import (
	"encoding/json"
	"errors"
)

func readTasks(data []byte) (map[string]task, error) {
	type instanceJson struct {
		Application_id        string
		Warden_job_id         uint64
		Warden_container_path string
		Instance_index        uint64
		State                 string
		Syslog_drain_urls     []string
	}

	type instancesJson struct {
		Instances []instanceJson
	}

	var jsonInstances instancesJson

	if len(data) < 1 {
		return nil, errors.New("Empty data, can't parse json")
	}

	err := json.Unmarshal(data, &jsonInstances)
	if err != nil {
		return nil, err
	}
	tasks := make(map[string]task, len(jsonInstances.Instances))
	for _, jsonInstance := range jsonInstances.Instances {
		if jsonInstance.State == "RUNNING" {
			task := task{
				applicationId:       jsonInstance.Application_id,
				wardenContainerPath: jsonInstance.Warden_container_path,
				wardenJobId:         jsonInstance.Warden_job_id,
				index:               jsonInstance.Instance_index,
				drainUrls:           jsonInstance.Syslog_drain_urls}
			tasks[task.identifier()] = task
		}
	}

	return tasks, nil
}
