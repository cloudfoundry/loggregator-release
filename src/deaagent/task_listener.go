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
	stdOutListener, stdErrListener *loggingstream.LoggingStream
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
	tl.Infof("Starting to listen to %v\n", tl.taskIdentifier)

	stdOutReaderChan := make(chan io.Reader)
	stdErrReaderChan := make(chan io.Reader)
	go tl.stdOutListener.FetchReader(stdOutReaderChan)
	go tl.stdErrListener.FetchReader(stdErrReaderChan)

	stdOutReader, ok := <-stdOutReaderChan
	if !ok {
		tl.Errorf("TaskListener.StartListening: could not open reader for STDOUT for task %s", tl.taskIdentifier)
		return
	}
	stdErrReader, ok := <-stdErrReaderChan
	if !ok {
		tl.Errorf("TaskListener.StartListening: could not open reader for STDERR for task %s", tl.taskIdentifier)
		return
	}

	go logs.ScanLogStream(tl.task.ApplicationId, tl.task.SourceName, strconv.FormatUint(tl.task.Index, 10), stdOutReader, tl.closeChan)
	go logs.ScanErrorLogStream(tl.task.ApplicationId, tl.task.SourceName, strconv.FormatUint(tl.task.Index, 10), stdErrReader, tl.closeChan)

	<-tl.closeChan
}

func (tl *TaskListener) StopListening() {
	tl.stdOutListener.Stop()
	tl.stdErrListener.Stop()
	close(tl.closeChan)
}
