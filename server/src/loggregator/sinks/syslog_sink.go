package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"math"
	"sync/atomic"
	"time"
)

type SyslogSink struct {
	logger           *gosteno.Logger
	appId            string
	drainUrl         string
	sentMessageCount *uint64
	sentByteCount    *uint64
	listenerChannel  chan *logmessage.Message
	syslogWriter     SyslogWriter
}

func NewSyslogSink(appId string, drainUrl string, givenLogger *gosteno.Logger, syslogWriter SyslogWriter) Sink {
	givenLogger.Debugf("Syslog Sink %s: Created for appId [%s]", drainUrl, appId)
	return &SyslogSink{
		appId:            appId,
		drainUrl:         drainUrl,
		logger:           givenLogger,
		sentMessageCount: new(uint64),
		sentByteCount:    new(uint64),
		listenerChannel:  make(chan *logmessage.Message),
		syslogWriter:     syslogWriter,
	}
}

func (s *SyslogSink) Run(closeChan chan Sink) {
	backoffStrategy := newExponentialRetryStrategy()
	numberOfTries := 0

	messageChannel := runNewRingBuffer(s, 10).GetOutputChannel()
	for {
		time.Sleep(backoffStrategy(numberOfTries))
		if !s.syslogWriter.IsConnected() {
			err := s.syslogWriter.Connect()
			if err != nil {
				s.logger.Warnf("Syslog Sink %s: Error when dialing out. Backing off. Err: %v", s.drainUrl, err)
				numberOfTries++
				continue
			}
			s.syslogWriter.SetConnected(true)
			defer s.syslogWriter.Close()
		}

		s.logger.Debugf("Syslog Sink %s: Waiting for activity\n", s.drainUrl)
		message, ok := <-messageChannel
		if !ok {
			s.logger.Debugf("Syslog Sink %s: Closed listener channel detected. Closing.\n", s.drainUrl)
			return
		}
		s.logger.Debugf("Syslog Sink %s: Got %d bytes. Sending data\n", s.drainUrl, message.GetRawMessageLength())

		var err error

		switch message.GetLogMessage().GetMessageType() {
		case logmessage.LogMessage_OUT:
			_, err = s.syslogWriter.WriteStdout(message.GetLogMessage().GetMessage())
		case logmessage.LogMessage_ERR:
			_, err = s.syslogWriter.WriteStderr(message.GetLogMessage().GetMessage())
		}
		if err != nil {
			s.logger.Debugf("Syslog Sink %s: Error when trying to send data to sink. Backing off. Err: %v\n", s.drainUrl, err)
			numberOfTries++
			s.syslogWriter.SetConnected(false)
		} else {
			s.logger.Debugf("Syslog Sink %s: Successfully sent data\n", s.drainUrl)
			numberOfTries = 0
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

type retryStrategy func(counter int) time.Duration

func newExponentialRetryStrategy() retryStrategy {
	exponential := func(counter int) time.Duration {
		duration := math.Pow(2, float64(counter))
		return time.Duration(int(duration)) * time.Millisecond
	}
	return exponential
}
