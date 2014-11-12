package loggingstream

import (
	"deaagent/domain"
	"errors"
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"io"
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
	close       chan struct{}
	sync.RWMutex
}

func NewLoggingStream(task *domain.Task, logger *gosteno.Logger, messageType events.LogMessage_MessageType) (ls *LoggingStream) {
	return &LoggingStream{task: task, logger: logger, messageType: messageType, close: make(chan struct{})}
}

func (ls *LoggingStream) Read(p []byte) (n int, err error) {
	ls.Lock()
	defer ls.Unlock()

	if ls.connection == nil {
		connection, err := ls.connect()
		if err != nil {
			ls.disconnect()
			return 0, io.EOF
		}
		ls.connection = connection
	}

	read := make(chan readReturn)

	go func() {
		n, err := ls.connection.Read(p)
		read <- readReturn{n: n, err: err}
	}()

	select {
	case <-ls.close:
		err = errors.New("Closed from outside")
	case readReturnValue := <-read:
		n = readReturnValue.n
		err = readReturnValue.err
	}

	if err != nil {
		endConnectionError := ls.disconnect()

		if endConnectionError != nil {
			err = errors.New(fmt.Sprintf("Error: %v\nClose Error: %v", err, endConnectionError))
		}
		ls.logger.Debugf("Stopped reading from socket %s", err.Error())
	}

	return
}

func (ls *LoggingStream) Close() error {
	close(ls.close)
	ls.RLock()
	defer ls.RUnlock()
	return ls.disconnect()
}

func (ls *LoggingStream) disconnect() error {
	if ls.connection == nil {
		return nil
	}

	err := ls.connection.Close()
	ls.connection = nil
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

type readReturn struct {
	n   int
	err error
}
