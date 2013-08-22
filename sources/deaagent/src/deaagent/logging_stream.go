package deaagent

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"net"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"
)

type loggingStream struct {
	inst              *instance
	loggregatorClient loggregatorclient.LoggregatorClient
	logger            *gosteno.Logger
	messageType       logmessage.LogMessage_MessageType
	messagesReceived  *uint64
	bytesReceived     *uint64
}

func newLoggingStream(inst *instance, loggregatorClient loggregatorclient.LoggregatorClient, logger *gosteno.Logger, messageType logmessage.LogMessage_MessageType) (ls *loggingStream) {
	return &loggingStream{inst, loggregatorClient, logger, messageType, new(uint64), new(uint64)}
}

func (ls *loggingStream) listen() {
	newLogMessage := func(message []byte) *logmessage.LogMessage {
		currentTime := time.Now()
		sourceType := logmessage.LogMessage_DEA
		return &logmessage.LogMessage{
			Message:     message,
			AppId:       proto.String(ls.inst.applicationId),
			MessageType: &ls.messageType,
			SourceType:  &sourceType,
			Timestamp:   proto.Int64(currentTime.UnixNano()),
		}
	}

	socket := func(messageType logmessage.LogMessage_MessageType) (net.Conn, error) {
		return net.Dial("unix", filepath.Join(ls.inst.identifier(), socketName(messageType)))
	}

	go func() {

		connection, err := socket(ls.messageType)
		if err != nil {
			ls.logger.Errorf("Error while dialing into socket %s, %s", ls.messageType, err)
			return
		}
		defer func() {
			connection.Close()
			ls.logger.Infof("Stopped reading from socket %s", ls.messageType)
		}()

		buffer := make([]byte, bufferSize)

		for {
			readCount, err := connection.Read(buffer)
			if readCount == 0 && err != nil {
				ls.logger.Infof("Error while reading from socket %s, %s", ls.messageType, err)
				break
			}
			ls.logger.Debugf("Read %d bytes from instance socket %s", readCount, ls.messageType)
			atomic.AddUint64(ls.messagesReceived, 1)
			atomic.AddUint64(ls.bytesReceived, uint64(readCount))
			ls.loggregatorClient.IncLogStreamRawByteCount(uint64(readCount))

			data, err := proto.Marshal(newLogMessage(buffer[0:readCount]))
			if err != nil {
				ls.logger.Errorf("Error marshalling log message: %s", err)
			}
			ls.loggregatorClient.IncLogStreamPbByteCount(uint64(len(data)))

			ls.loggregatorClient.Send(data)
			ls.logger.Debugf("Sent %d bytes to loggregator client", readCount)
			runtime.Gosched()
		}
	}()
}

func (ls *loggingStream) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "loggingStream:" + ls.inst.wardenContainerPath + " type " + socketName(ls.messageType),
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "receivedMessageCount", Value: atomic.LoadUint64(ls.messagesReceived)},
			instrumentation.Metric{Name: "receivedByteCount", Value: atomic.LoadUint64(ls.bytesReceived)},
		},
	}
}

func socketName(messageType logmessage.LogMessage_MessageType) string {
	if messageType == logmessage.LogMessage_OUT {
		return "stdout.sock"
	} else {
		return "stderr.sock"
	}
}
