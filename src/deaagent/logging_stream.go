package deaagent

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/emitter"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

type loggingStream struct {
	inst             instance
	emitter          emitter.Emitter
	logger           *gosteno.Logger
	messageType      logmessage.LogMessage_MessageType
	messagesReceived *uint64
	bytesReceived    *uint64
}

const bufferSize = 4096

func newLoggingStream(inst instance, emitter emitter.Emitter, logger *gosteno.Logger, messageType logmessage.LogMessage_MessageType) (ls *loggingStream) {
	return &loggingStream{inst, emitter, logger, messageType, new(uint64), new(uint64)}
}

func (ls loggingStream) listen() {
	newLogMessage := func(message []byte) *logmessage.LogMessage {
		currentTime := time.Now()
		sourceType := logmessage.LogMessage_WARDEN_CONTAINER
		sourceId := strconv.FormatUint(ls.inst.index, 10)

		return &logmessage.LogMessage{
			Message:     message,
			AppId:       proto.String(ls.inst.applicationId),
			DrainUrls:   ls.inst.drainUrls,
			MessageType: &ls.messageType,
			SourceType:  &sourceType,
			SourceId:    &sourceId,
			Timestamp:   proto.Int64(currentTime.UnixNano()),
		}
	}

	socket := func(messageType logmessage.LogMessage_MessageType) (net.Conn, error) {
		return net.Dial("unix", filepath.Join(ls.inst.identifier(), socketName(messageType)))
	}

	go func() {
		var connection net.Conn
		i := 0
		for {
			var err error
			connection, err = socket(ls.messageType)
			if err == nil {
				break
			} else {
				ls.logger.Errorf("Error while dialing into socket %s, %s, %s", ls.messageType, ls.inst.identifier(), err)
				i += 1
				if i < 86400 {
					time.Sleep(1 * time.Second)
				} else {
					ls.logger.Errorf("Giving up after %d tries dialing into socket %s, %s, %s", i, ls.messageType, ls.inst.identifier(), err)
					return
				}
			}
		}

		defer func() {
			connection.Close()
			ls.logger.Infof("Stopped reading from socket %s, %s", ls.messageType, ls.inst.identifier())
		}()

		buffer := make([]byte, bufferSize)

		for {
			readCount, err := connection.Read(buffer)
			if err != nil {
				ls.logger.Infof("Error while reading from socket %s, %s, %s", ls.messageType, ls.inst.identifier(), err)
				break
			}

			ls.logger.Debugf("Read %d bytes from instance socket %s, %s", readCount, ls.messageType, ls.inst.identifier())
			atomic.AddUint64(ls.messagesReceived, 1)
			atomic.AddUint64(ls.bytesReceived, uint64(readCount))

			rawMessageBytes := make([]byte, readCount)
			copy(rawMessageBytes, buffer[:readCount])
			logMessage := newLogMessage(rawMessageBytes)

			ls.emitter.EmitLogMessage(logMessage)

			ls.logger.Debugf("Sent %d bytes to loggregator client from %s, %s", readCount, ls.messageType, ls.inst.identifier())
			runtime.Gosched()
		}
	}()
}

func (ls loggingStream) Emit() instrumentation.Context {
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
