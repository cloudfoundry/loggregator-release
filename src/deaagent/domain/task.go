package domain

import (
	"path/filepath"
	"strconv"
)

type Task struct {
	ApplicationId       string
	Index               uint64
	WardenJobId         uint64
	WardenContainerPath string
	SourceName          string
}

func (task *Task) Identifier() string {
	return filepath.Join(task.WardenContainerPath, "jobs", strconv.FormatUint(task.WardenJobId, 10))
}
