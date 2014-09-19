package loggingstream

import (
	"code.google.com/p/gogoprotobuf/proto"
	"deaagent/domain"
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type LoggingStream struct {
	connection       net.Conn
	task             *domain.Task
	logger           *gosteno.Logger
	messageType      events.LogMessage_MessageType
	messagesReceived uint64
	bytesReceived    uint64
	sync.Mutex
}

func NewLoggingStream(task *domain.Task, logger *gosteno.Logger, messageType events.LogMessage_MessageType) (ls *LoggingStream) {
	return &LoggingStream{task: task, logger: logger, messageType: messageType}
}

func (ls *LoggingStream) FetchReader(returnChan chan io.Reader) {
	connection, err := ls.connect()
	if err != nil {
		ls.logger.Infof("Error while reading from socket %s, %s, %s", ls.messageType, ls.task.Identifier(), err)
		close(returnChan)
		return
	}
	ls.setConnection(connection)

	returnChan <- connection
	close(returnChan)
}

func (ls *LoggingStream) setConnection(connection net.Conn) {
	ls.Lock()
	defer ls.Unlock()
	ls.connection = connection
}

func (ls *LoggingStream) Stop() {
	ls.Lock()
	defer ls.Unlock()

	if ls.connection != nil {
		ls.connection.Close()
	}
	ls.logger.Infof("Stopped reading from socket %s, %s", ls.messageType, ls.task.Identifier())
}

func (ls *LoggingStream) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "loggingStream:" + ls.task.WardenContainerPath + " type " + socketName(ls.messageType),
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "receivedMessageCount", Value: atomic.LoadUint64(&ls.messagesReceived)},
			instrumentation.Metric{Name: "receivedByteCount", Value: atomic.LoadUint64(&ls.bytesReceived)},
		},
	}
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

func (ls *LoggingStream) newLogMessage(message []byte) *events.LogMessage {
	currentTime := time.Now()
	sourceName := ls.task.SourceName
	sourceId := strconv.FormatUint(ls.task.Index, 10)
	messageCopy := make([]byte, len(message))
	copyCount := copy(messageCopy, message)
	if copyCount != len(message) {
		panic(fmt.Sprintf("Didn't copy the message %d, %s", copyCount, message))
	}
	return &events.LogMessage{
		Message:        messageCopy,
		AppId:          proto.String(ls.task.ApplicationId),
		MessageType:    &ls.messageType,
		SourceType:     &sourceName,
		SourceInstance: &sourceId,
		Timestamp:      proto.Int64(currentTime.UnixNano()),
	}
}
