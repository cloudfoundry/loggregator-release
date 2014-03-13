package deaagent

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"path/filepath"
	"strconv"
)

type Task struct {
	ApplicationId                  string
	DrainUrls                      []string
	Index                          uint64
	WardenJobId                    uint64
	WardenContainerPath            string
	SourceName                     string
	stdOutListener, stdErrListener *LoggingStream
}

func (task *Task) Identifier() string {
	return filepath.Join(task.WardenContainerPath, "jobs", strconv.FormatUint(task.WardenJobId, 10))
}

func (task *Task) startListening(e emitter.Emitter, logger *gosteno.Logger) {
	logger.Infof("Starting to listen to %v\n", task.Identifier())

	task.stdOutListener = NewLoggingStream(task, logger, logmessage.LogMessage_OUT)
	task.stdErrListener = NewLoggingStream(task, logger, logmessage.LogMessage_ERR)

	stdOutChan := task.stdOutListener.Listen()
	stdErrChan := task.stdErrListener.Listen()

	for {
		select {
		case msg, ok := <-stdOutChan:
			if !ok {
				task.stdErrListener.Stop()
				return
			}
			e.EmitLogMessage(msg)
		case msg, ok := <-stdErrChan:
			if !ok {
				task.stdOutListener.Stop()
				return
			}
			e.EmitLogMessage(msg)
		}
	}
}

func (task *Task) stopListening() {
	task.stdOutListener.Stop()
	task.stdErrListener.Stop()
}
