package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"net/url"
	"sync/atomic"
)

type syslogSink struct {
	logger           *gosteno.Logger
	appId            string
	drainUrl         string
	sysLogger        *writer
	sentMessageCount *uint64
	sentByteCount    *uint64
	listenerChannel  chan *logmessage.Message
}

func NewSyslogSink(appId string, drainUrl string, givenLogger *gosteno.Logger) (Sink, error) {

	dl, err := url.Parse(drainUrl)
	if err != nil {
		return nil, err
	}
	sysLogger, err := dial("tcp", dl.Host, appId, givenLogger)
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
		listenerChannel:  make(chan *logmessage.Message),
	}, nil
}

func (sink *syslogSink) Run(closeChan chan Sink) {
	alreadyRequestedClose := false
	defer sink.sysLogger.close()

	for {
		sink.logger.Debugf("Syslog Sink %s: Waiting for activity", sink.drainUrl)
		message, ok := <-sink.listenerChannel
		if !ok {
			sink.logger.Debugf("Syslog Sink %s: Closed listener channel detected. Closing.", sink.drainUrl)
			return
		}
		sink.logger.Debugf("Syslog Sink %s: Got %d bytes. Sending data", sink.drainUrl, message.GetRawMessageLength())

		var err error

		switch message.GetLogMessage().GetMessageType() {
		case logmessage.LogMessage_OUT:
			_, err = sink.sysLogger.writeStdout(message.GetRawMessage())
		case logmessage.LogMessage_ERR:
			_, err = sink.sysLogger.writeStderr(message.GetRawMessage())
		}
		if err != nil {
			sink.logger.Debugf("Syslog Sink %s: Error when trying to send data to sink. Requesting close. Err: %v", sink.drainUrl, err)
			if !alreadyRequestedClose {
				alreadyRequestedClose = true
				closeChan <- sink
				sink.logger.Debugf("Syslog Sink %s: Successfully requested listener channel close", sink.drainUrl)
			} else {
				sink.logger.Debugf("Syslog Sink %s: Previously requested close. Doing nothing", sink.drainUrl)
			}
		} else {
			sink.logger.Debugf("Syslog Sink %s: Successfully sent data", sink.drainUrl)
			atomic.AddUint64(sink.sentMessageCount, 1)
			atomic.AddUint64(sink.sentByteCount, uint64(message.GetRawMessageLength()))
		}
	}
}

func (s *syslogSink) ListenerChannel() chan *logmessage.Message {
	return s.listenerChannel
}

func (s *syslogSink) Identifier() string {
	return s.drainUrl
}

func (s *syslogSink) AppId() string {
	return s.appId
}

func (sink *syslogSink) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "syslogSink",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "sentMessageCount:" + sink.appId, Value: atomic.LoadUint64(sink.sentMessageCount)},
			instrumentation.Metric{Name: "sentByteCount:" + sink.appId, Value: atomic.LoadUint64(sink.sentByteCount)},
		},
	}
}
