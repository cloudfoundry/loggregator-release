package dea_logging_agent

import "path/filepath"

type Instance struct {
	ApplicationId string
	WardenJobId string
	WardenContainerPath string
}

func (instance *Instance) Identifier() (string) {
	return filepath.Join(instance.WardenContainerPath, "jobs", instance.WardenJobId)
}
