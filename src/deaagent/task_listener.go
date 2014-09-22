package deaagent

import (
	"deaagent/domain"
	"deaagent/loggingstream"
	"github.com/cloudfoundry/dropsonde/autowire/logs"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"io"
	"strconv"
)

type TaskListener struct {
	*gosteno.Logger
	taskIdentifier                 string
	stdOutListener, stdErrListener io.ReadCloser
	task                           domain.Task
	closeChan                      chan struct{}
}

func NewTaskListener(task domain.Task, logger *gosteno.Logger) *TaskListener {
	return &TaskListener{
		Logger:         logger,
		taskIdentifier: task.Identifier(),
		stdOutListener: loggingstream.NewLoggingStream(&task, logger, events.LogMessage_OUT),
		stdErrListener: loggingstream.NewLoggingStream(&task, logger, events.LogMessage_ERR),
		task:           task,
		closeChan:      make(chan struct{}),
	}
}

func (tl *TaskListener) Task() domain.Task {
	return tl.task
}

func (tl *TaskListener) StartListening() {
	tl.Debugf("TaskListener.StartListening: Starting to listen to %v\n", tl.taskIdentifier)
	tl.Debugf("TaskListener.StartListening: Scanning logs for %s", tl.task.ApplicationId)
	go logs.ScanLogStream(tl.task.ApplicationId, tl.task.SourceName, strconv.FormatUint(tl.task.Index, 10), tl.stdOutListener, tl.closeChan)
	go logs.ScanErrorLogStream(tl.task.ApplicationId, tl.task.SourceName, strconv.FormatUint(tl.task.Index, 10), tl.stdErrListener, tl.closeChan)

	<-tl.closeChan
}

func (tl *TaskListener) StopListening() {
	tl.stdOutListener.Close()
	tl.stdErrListener.Close()
	tl.Debugf("TaskListener.StopListening: Shutting down logs for %s", tl.task.ApplicationId)
	close(tl.closeChan)
}
