package deaagent

import (
	"deaagent/domain"
	"deaagent/loggingstream"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
)

type TaskListener struct {
	*gosteno.Logger
	emitter                        emitter.Emitter
	taskIdentifier                 string
	stdOutListener, stdErrListener *loggingstream.LoggingStream
}

func NewTaskListener(task domain.Task, e emitter.Emitter, logger *gosteno.Logger) *TaskListener {
	return &TaskListener{
		Logger:         logger,
		emitter:        e,
		taskIdentifier: task.Identifier(),
		stdOutListener: loggingstream.NewLoggingStream(&task, logger, logmessage.LogMessage_OUT),
		stdErrListener: loggingstream.NewLoggingStream(&task, logger, logmessage.LogMessage_ERR),
	}
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
