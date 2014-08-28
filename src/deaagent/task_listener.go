package deaagent

import (
	"deaagent/domain"
	"deaagent/loggingstream"
	"github.com/cloudfoundry/dropsonde/emitter/logemitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
)

type TaskListener struct {
	*gosteno.Logger
	emitter                        logemitter.Emitter
	taskIdentifier                 string
	stdOutListener, stdErrListener *loggingstream.LoggingStream
	task                           domain.Task
}

func NewTaskListener(task domain.Task, e logemitter.Emitter, logger *gosteno.Logger) *TaskListener {
	return &TaskListener{
		Logger:         logger,
		emitter:        e,
		taskIdentifier: task.Identifier(),
		stdOutListener: loggingstream.NewLoggingStream(&task, logger, events.LogMessage_OUT),
		stdErrListener: loggingstream.NewLoggingStream(&task, logger, events.LogMessage_ERR),
		task:           task,
	}
}

func (tl *TaskListener) Task() domain.Task {
	return tl.task
}

func (tl *TaskListener) StartListening() {
	tl.Infof("Starting to listen to %v\n", tl.taskIdentifier)

	stdOutChan := tl.stdOutListener.Listen()
	stdErrChan := tl.stdErrListener.Listen()

	for {
		select {
		case msg, ok := <-stdOutChan:
			if !ok {
				tl.stdErrListener.Stop()
				return
			}
			tl.emitter.EmitLogMessage(msg)
		case msg, ok := <-stdErrChan:
			if !ok {
				tl.stdOutListener.Stop()
				return
			}
			tl.emitter.EmitLogMessage(msg)
		}
	}
}

func (tl *TaskListener) StopListening() {
	tl.stdOutListener.Stop()
	tl.stdErrListener.Stop()
}
