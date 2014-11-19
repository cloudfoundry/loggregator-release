package loggingstream

import (
	"deaagent/domain"
	"errors"
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"net"
	"path/filepath"
	"sync"
	"time"
)

type LoggingStream struct {
	connection  net.Conn
	task        *domain.Task
	logger      *gosteno.Logger
	messageType events.LogMessage_MessageType
	sync.RWMutex
}

func NewLoggingStream(task *domain.Task, logger *gosteno.Logger, messageType events.LogMessage_MessageType) (ls *LoggingStream) {
	ls = &LoggingStream{task: task, logger: logger, messageType: messageType }

	connection, err := ls.connect()
	if err != nil {
		return nil
	}

	ls.connection = connection

	return ls
}

func (ls *LoggingStream) Read(p []byte) (n int, err error) {
	n, err = ls.connection.Read(p)

	if err != nil {
		endConnectionError := ls.Close()

		if endConnectionError != nil {
			err = errors.New(fmt.Sprintf("Error: %v\nClose Error: %v", err, endConnectionError))
		}
		ls.logger.Debugf("Stopped reading from socket %s", err.Error())
	}

	return n, err
}

func (ls *LoggingStream) Close() error {
	err := ls.connection.Close()
	ls.logger.Debugf("Stopped reading from socket %s, %s", ls.messageType, ls.task.Identifier())
	return err
}

func (ls *LoggingStream) connect() (net.Conn, error) {
	var err error
	var connection net.Conn
	for i := 0; i < 10; i++ {
		connection, err = net.Dial("unix", filepath.Join(ls.task.Identifier(), socketName(ls.messageType)))
		if err == nil {
			ls.logger.Debugf("Opened socket %s, %s", ls.messageType, ls.task.Identifier())
			break
		}
		ls.logger.Debugf("Could not read from socket %s, %s, retrying: %s", ls.messageType, ls.task.Identifier(), err)
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
