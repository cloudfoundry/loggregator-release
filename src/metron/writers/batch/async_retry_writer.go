package batch

import (
	"fmt"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type AsyncRetryWriter interface {
	AsyncWrite(bytes []byte, retry int, messageCount uint64)
}

type asyncRetryWriter struct {
	writer    ByteWriter
	errWriter ByteWriter
	logger    *gosteno.Logger
}

func NewAsyncRetryWriter(writer, errWriter ByteWriter, logger *gosteno.Logger) *asyncRetryWriter {
	return &asyncRetryWriter{
		writer:    writer,
		errWriter: errWriter,
		logger:    logger,
	}
}

func (aw *asyncRetryWriter) AsyncWrite(bytes []byte, retry int, messageCount uint64) {
	go func() {
		var err error

		for i := 0; i < retry+1; i++ {
			if i > 0 {
				metrics.BatchIncrementCounter("DopplerForwarder.retryCount")
			}
			_, err = aw.writer.Write(bytes)
			if err == nil {
				metrics.BatchAddCounter("DopplerForwarder.sentMessages", messageCount)
				return
			}
		}

		metrics.BatchAddCounter("MessageBuffer.droppedMessageCount", messageCount)
		aw.logger.Warnf("Received error while trying to flush TCP bytes: %s", err)
		_, err = aw.errWriter.Write(aw.droppedLogMessage(messageCount))
		if err != nil {
			aw.logger.Warnf("Unable to write droppedLogMessage into batch writer: %s", err)
		}
	}()
}

func (aw *asyncRetryWriter) droppedLogMessage(droppedMessages uint64) []byte {
	logMessage := &events.LogMessage{
		Message:     []byte(fmt.Sprintf("Dropped %d message(s) from MetronAgent to Doppler", droppedMessages)),
		MessageType: events.LogMessage_ERR.Enum(),
		AppId:       proto.String(envelope_extensions.SystemAppId),
		Timestamp:   proto.Int64(time.Now().UnixNano()),
	}
	env, err := emitter.Wrap(logMessage, "MetronAgent")
	if err != nil {
		aw.logger.Fatalf("Failed to emitter.Wrap a log message: %s", err)
	}
	logMsg, err := proto.Marshal(env)
	if err != nil {
		aw.logger.Fatalf("Failed to proto.Marshal log message: %s", err)
	}
	return logMsg
}
