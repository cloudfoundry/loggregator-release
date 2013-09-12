package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"net/url"
	"sync/atomic"
)

type SyslogSink struct {
	logger           *gosteno.Logger
	appId            string
	drainUrl         string
	sentMessageCount *uint64
	sentByteCount    *uint64
	listenerChannel  chan *logmessage.Message
}

func NewSyslogSink(appId string, drainUrl string, givenLogger *gosteno.Logger) Sink {
	givenLogger.Debugf("Syslog Sink %s: Created for appId [%s]", drainUrl, appId)
	return &SyslogSink{
		appId:            appId,
		drainUrl:         drainUrl,
		logger:           givenLogger,
		sentMessageCount: new(uint64),
		sentByteCount:    new(uint64),
		listenerChannel:  make(chan *logmessage.Message),
	}
}

func (s *SyslogSink) Run(closeChan chan Sink) {
	alreadyRequestedClose := false

	dl, err := url.Parse(s.drainUrl)
	if err != nil {
		s.logger.Warnf("Syslog Sink %s: Error when trying to parse syslog url. Requesting close. Err: %v", s.drainUrl, err)
		requestClose(s, closeChan, &alreadyRequestedClose)
	}
	sysLogger, err := dial("tcp", dl.Host, s.appId, s.logger)
	defer sysLogger.close()
	if err != nil {
		s.logger.Warnf("Syslog Sink %s: Error when dialing out. Requesting close. Err: %v", s.drainUrl, err)
		requestClose(s, closeChan, &alreadyRequestedClose)
	}

	messageChannel := newRingBufferChannel(s)
	for {
		s.logger.Debugf("Syslog Sink %s: Waiting for activity", s.drainUrl)
		message, ok := <-messageChannel
		if !ok {
			s.logger.Debugf("Syslog Sink %s: Closed listener channel detected. Closing.", s.drainUrl)
			return
		}
		s.logger.Debugf("Syslog Sink %s: Got %d bytes. Sending data", s.drainUrl, message.GetRawMessageLength())

		var err error

		switch message.GetLogMessage().GetMessageType() {
		case logmessage.LogMessage_OUT:
			_, err = sysLogger.writeStdout(message.GetLogMessage().GetMessage())
		case logmessage.LogMessage_ERR:
			_, err = sysLogger.writeStderr(message.GetLogMessage().GetMessage())
		}
		if err != nil {
			s.logger.Debugf("Syslog Sink %s: Error when trying to send data to sink. Requesting close. Err: %v", s.drainUrl, err)
			requestClose(s, closeChan, &alreadyRequestedClose)
		} else {
			s.logger.Debugf("Syslog Sink %s: Successfully sent data", s.drainUrl)
			atomic.AddUint64(s.sentMessageCount, 1)
			atomic.AddUint64(s.sentByteCount, uint64(message.GetRawMessageLength()))
		}
	}
}

func (s *SyslogSink) Channel() chan *logmessage.Message {
	return s.listenerChannel
}

func (s *SyslogSink) Logger() *gosteno.Logger {
	return s.logger
}

func (s *SyslogSink) Identifier() string {
	return s.drainUrl
}

func (s *SyslogSink) AppId() string {
	return s.appId
}

func (s *SyslogSink) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "syslogSink",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "sentMessageCount:" + s.appId, Value: atomic.LoadUint64(s.sentMessageCount)},
			instrumentation.Metric{Name: "sentByteCount:" + s.appId, Value: atomic.LoadUint64(s.sentByteCount)},
		},
	}
}
