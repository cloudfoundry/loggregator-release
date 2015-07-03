package deaagent

import (
	"deaagent/domain"
	"errors"
	"fmt"
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type TaskListener struct {
	*gosteno.Logger
	taskIdentifier             string
	stdOutReader, stdErrReader io.ReadCloser
	task                       domain.Task
	sync.WaitGroup
}

func NewTaskListener(task domain.Task, logger *gosteno.Logger) (*TaskListener, error) {
	stdOutReader, err := dial(task.Identifier(), events.LogMessage_OUT, logger)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Connection to stdout %s failed\n", task.Identifier()))
	}
	stdErrReader, err := dial(task.Identifier(), events.LogMessage_ERR, logger)
	if err != nil {
		stdOutReader.Close()
		return nil, errors.New(fmt.Sprintf("Connection to stderr %s failed\n", task.Identifier()))
	}
	return &TaskListener{
		Logger:         logger,
		taskIdentifier: task.Identifier(),
		stdOutReader:   stdOutReader,
		stdErrReader:   stdErrReader,
		task:           task,
	}, nil
}

func (tl *TaskListener) Task() domain.Task {
	return tl.task
}

func (tl *TaskListener) StartListening() {
	tl.Debugf("TaskListener.StartListening: Starting to listen to %v\n", tl.taskIdentifier)
	tl.Debugf("TaskListener.StartListening: Scanning logs for %s", tl.task.ApplicationId)

	tl.Add(2)
	go func() {
		defer tl.Done()
		defer tl.StopListening()
		logs.ScanLogStream(tl.task.ApplicationId, tl.task.SourceName, strconv.FormatUint(tl.task.Index, 10), tl.stdOutReader)
	}()
	go func() {
		defer tl.Done()
		defer tl.StopListening()
		logs.ScanErrorLogStream(tl.task.ApplicationId, tl.task.SourceName, strconv.FormatUint(tl.task.Index, 10), tl.stdErrReader)
	}()

	tl.Wait()
}

func (tl *TaskListener) StopListening() {
	tl.stdOutReader.Close()
	tl.stdErrReader.Close()
	tl.Debugf("TaskListener.StopListening: Shutting down logs for %s", tl.task.ApplicationId)
}

func dial(taskIdentifier string, messageType events.LogMessage_MessageType, logger *gosteno.Logger) (net.Conn, error) {
	var err error
	var connection net.Conn
	for i := 0; i < 10; i++ {
		connection, err = net.Dial("unix", filepath.Join(taskIdentifier, socketName(messageType)))
		if err == nil {
			logger.Debugf("Opened socket %s, %s", messageType, taskIdentifier)
			break
		}
		logger.Debugf("Could not read from socket %s, %s, retrying: %s", messageType, taskIdentifier, err)
		time.Sleep(100 * time.Millisecond)
	}
	return connection, err
}

func socketName(messageType events.LogMessage_MessageType) string {
	if messageType == events.LogMessage_OUT {
		return "stdout.sock"
	}
	return "stderr.sock"
}
