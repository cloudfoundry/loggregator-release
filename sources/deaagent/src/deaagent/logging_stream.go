package deaagent

import (
	"code.google.com/p/gogoprotobuf/proto"
	"deaagent/loggregatorclient"
	"github.com/cloudfoundry/gosteno"
	"instrumentor"
	"logMessage"
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

type loggingStream struct {
	inst              *instance
	loggregatorClient loggregatorclient.LoggregatorClient
	logger            *gosteno.Logger
	messageType       logMessage.LogMessage_MessageType
	messagesReceived  *uint64
	bytesReceived     *uint64
}

func newLoggingStream(inst *instance, loggregatorClient loggregatorclient.LoggregatorClient, logger *gosteno.Logger, messageType logMessage.LogMessage_MessageType) (ls *loggingStream) {
	return &loggingStream{inst, loggregatorClient, logger, messageType, new(uint64), new(uint64)}
}

func (ls *loggingStream) listen() {
	lsInstrumentor := instrumentor.NewInstrumentor(5*time.Second, gosteno.LOG_DEBUG, ls.logger)
	stopChan := lsInstrumentor.Instrument(ls)

	newLogMessage := func(message []byte) *logMessage.LogMessage {
		currentTime := time.Now()
		sourceType := logMessage.LogMessage_DEA
		return &logMessage.LogMessage{
			Message:     message,
			AppId:       proto.String(ls.inst.applicationId),
			SpaceId:     proto.String(ls.inst.spaceId),
			MessageType: &ls.messageType,
			SourceType:  &sourceType,
			Timestamp:   proto.Int64(currentTime.UnixNano()),
		}
	}

	socket := func(messageType logMessage.LogMessage_MessageType) (net.Conn, error) {
		return net.Dial("unix", filepath.Join(ls.inst.identifier(), socketName(messageType)))
	}

	go func() {
		defer lsInstrumentor.StopInstrumentation(stopChan)

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
			ls.logger.Debugf("Read %d bytes from instance socket", readCount)

			data, err := proto.Marshal(newLogMessage(buffer[0:readCount]))
			if err != nil {
				ls.logger.Errorf("Error marshalling log message: %s", err)
			}

			ls.loggregatorClient.Send(data)
			atomic.AddUint64(ls.messagesReceived, 1)
			atomic.AddUint64(ls.bytesReceived, uint64(readCount))
			ls.logger.Debugf("Sent %d bytes to loggregator client", readCount)
			runtime.Gosched()
		}
	}()
}

func (ls *loggingStream) DumpData() []instrumentor.PropVal {
	messagesProperty := "ReceivedMessageCount from " + ls.inst.wardenContainerPath + " type " + socketName(ls.messageType)
	bytesProperty := "ReceivedByteCount from " + ls.inst.wardenContainerPath + " type " + socketName(ls.messageType)

	return []instrumentor.PropVal{
		instrumentor.PropVal{messagesProperty, strconv.FormatUint(atomic.LoadUint64(ls.messagesReceived), 10)},
		instrumentor.PropVal{bytesProperty, strconv.FormatUint(atomic.LoadUint64(ls.bytesReceived), 10)},
	}
}

func socketName(messageType logMessage.LogMessage_MessageType) string {
	if messageType == logMessage.LogMessage_OUT {
		return "stdout.sock"
	} else {
		return "stderr.sock"
	}
}
