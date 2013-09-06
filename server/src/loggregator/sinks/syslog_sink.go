package sinks

import (
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"sync/atomic"
)

type syslogSink struct {
	logger           *gosteno.Logger
	appId            string
	drainUrl         string
	sysLogger        *writer
	sentMessageCount *uint64
	sentByteCount    *uint64
	listenerChannel  chan []byte
}

func NewSyslogSink(appId string, drainUrl string, givenLogger *gosteno.Logger) (Sink, error) {
	sysLogger, err := dial("tcp", drainUrl, appId)
	if err != nil {
		return nil, err
	}
	givenLogger.Debugf("Syslog Sink %s: Created for appId [%s]", drainUrl, appId)
	return &syslogSink{
		appId:            appId,
		sysLogger:        sysLogger,
		drainUrl:         drainUrl,
		logger:           givenLogger,
		sentMessageCount: new(uint64),
		sentByteCount:    new(uint64),
		listenerChannel:  make(chan []byte),
	}, nil
}

func (sink *syslogSink) Run(closeChan chan chan []byte) {
	alreadyRequestedClose := false
	defer sink.sysLogger.close()

	for {
		sink.logger.Debugf("Syslog Sink %s: Waiting for activity", sink.drainUrl)
		data, ok := <-sink.listenerChannel
		if !ok {
			sink.logger.Debugf("Syslog Sink %s: Closed listener channel detected. Closing.", sink.drainUrl)
			return
		}
		sink.logger.Debugf("Syslog Sink %s: Got %d bytes. Sending data", sink.drainUrl, len(data))
		message := new(logmessage.LogMessage)
		err := proto.Unmarshal(data, message)
		if err != nil {
			sink.logger.Debugf("Syslog Sink %s: Could not unmarshal a logmessage dropping it, %v", sink.drainUrl, err)
			continue
		}
		switch message.GetMessageType() {
		case logmessage.LogMessage_OUT:
			_, err = sink.sysLogger.writeStdout(message.GetMessage())
		case logmessage.LogMessage_ERR:
			_, err = sink.sysLogger.writeStderr(message.GetMessage())
		}
		if err != nil {
			sink.logger.Debugf("Syslog Sink %s: Error when trying to send data to sink %s. Requesting close. Err: %v", sink.drainUrl, err)
			if !alreadyRequestedClose {
				closeChan <- sink.listenerChannel
				alreadyRequestedClose = true
				sink.logger.Debugf("Syslog Sink %s: Successfully requested listener channel close", sink.drainUrl)
			} else {
				sink.logger.Debugf("Syslog Sink %s: Previously requested close. Doing nothing", sink.drainUrl)
			}
		} else {
			sink.logger.Debugf("Syslog Sink %s: Successfully sent data", sink.drainUrl)
			atomic.AddUint64(sink.sentMessageCount, 1)
			atomic.AddUint64(sink.sentByteCount, uint64(len(data)))
		}
	}
}

func (s *syslogSink) ListenerChannel() chan []byte {
	return s.listenerChannel
}

func (s *syslogSink) Identifier() string {
	return s.drainUrl
}

func (sink *syslogSink) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "syslogSink",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "sentMessageCount:" + sink.appId, Value: atomic.LoadUint64(sink.sentMessageCount)},
			instrumentation.Metric{Name: "sentByteCount:" + sink.appId, Value: atomic.LoadUint64(sink.sentByteCount)},
		},
	}
}
