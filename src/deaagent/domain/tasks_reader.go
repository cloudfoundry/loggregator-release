package domain

import (
	"encoding/json"
	"errors"
)

func ReadTasks(data []byte) (map[string]Task, error) {
	type instanceJson struct {
		Application_id        string
		Warden_job_id         uint64
		Warden_container_path string
		Instance_index        uint64
		State                 string
	}

	type stagingMessageJson struct {
		App_id string
	}

	type stagingTaskJson struct {
		Staging_message       stagingMessageJson
		Warden_job_id         uint64
		Warden_container_path string
	}

	type instancesJson struct {
		Instances     []instanceJson
		Staging_tasks []stagingTaskJson
	}

	var jsonInstances instancesJson

	if len(data) < 1 {
		return nil, errors.New("Empty data, can't parse json")
	}

	err := json.Unmarshal(data, &jsonInstances)
	if err != nil {
		return nil, err
	}
	tasks := make(map[string]Task, len(jsonInstances.Instances))
	for _, jsonInstance := range jsonInstances.Instances {
		if jsonInstance.Warden_container_path == "" || jsonInstance.Warden_job_id == 0 {
			continue
		}
		if isStateTracked(jsonInstance.State) {
			task := Task{
				ApplicationId:       jsonInstance.Application_id,
				SourceName:          "App",
				WardenContainerPath: jsonInstance.Warden_container_path,
				WardenJobId:         jsonInstance.Warden_job_id,
				Index:               jsonInstance.Instance_index,
			}
			tasks[task.Identifier()] = task
		}
	}

	for _, jsonStagingTask := range jsonInstances.Staging_tasks {
		if jsonStagingTask.Warden_job_id == 0 {
			continue
		}
		task := Task{
			ApplicationId:       jsonStagingTask.Staging_message.App_id,
			SourceName:          "STG",
			WardenContainerPath: jsonStagingTask.Warden_container_path,
			WardenJobId:         jsonStagingTask.Warden_job_id,
		}
		tasks[task.Identifier()] = task
	}

	return tasks, nil
}

func isStateTracked(state string) bool {
	return (state == "RUNNING" || state == "STARTING" || state == "STOPPING")
}
