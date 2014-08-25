package syslog

import (
	"code.google.com/p/gogoprotobuf/proto"
	"doppler/envelopewrapper"
	"doppler/sinks"
	"doppler/sinks/retrystrategy"
	"doppler/sinks/syslogwriter"
	"fmt"
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"sync/atomic"
	"time"
)

type SyslogSink struct {
	logger            *gosteno.Logger
	appId             string
	drainUrl          string
	sentMessageCount  *uint64
	sentByteCount     *uint64
	listenerChannel   chan *envelopewrapper.WrappedEnvelope
	syslogWriter      syslogwriter.SyslogWriter
	errorChannel      chan<- *envelopewrapper.WrappedEnvelope
	disconnectChannel chan struct{}
}

func NewSyslogSink(appId string, drainUrl string, givenLogger *gosteno.Logger, syslogWriter syslogwriter.SyslogWriter, errorChannel chan<- *envelopewrapper.WrappedEnvelope) sinks.Sink {
	givenLogger.Debugf("Syslog Sink %s: Created for appId [%s]", drainUrl, appId)
	return &SyslogSink{
		appId:             appId,
		drainUrl:          drainUrl,
		logger:            givenLogger,
		sentMessageCount:  new(uint64),
		sentByteCount:     new(uint64),
		syslogWriter:      syslogWriter,
		errorChannel:      errorChannel,
		disconnectChannel: make(chan struct{}),
	}
}

func (s *SyslogSink) Run(inputChan <-chan *envelopewrapper.WrappedEnvelope) {
	s.logger.Infof("Syslog Sink %s: Running.", s.drainUrl)
	defer s.logger.Errorf("Syslog Sink %s: Stopped.", s.drainUrl)

	backoffStrategy := retrystrategy.NewExponentialRetryStrategy()
	numberOfTries := 0

	buffer := sinks.RunTruncatingBuffer(inputChan, 100, s.logger)
	timer := time.NewTimer(backoffStrategy(numberOfTries))
	defer timer.Stop()
	for {
		s.logger.Debugf("Syslog Sink %s: Starting loop. Current backoff: %v", s.drainUrl, backoffStrategy(numberOfTries))
		timer.Reset(backoffStrategy(numberOfTries))
		select {
		case <-s.disconnectChannel:
			return
		case <-timer.C:
		}

		if !s.syslogWriter.IsConnected() {
			s.logger.Debugf("Syslog Sink %s: Not connected. Trying to connect.", s.drainUrl)
			err := s.syslogWriter.Connect()
			if err != nil {
				errorMsg := fmt.Sprintf("Syslog Sink %s: Error when dialing out. Backing off for %v. Err: %v", s.drainUrl, backoffStrategy(numberOfTries+1), err)
				numberOfTries++

				s.logger.Warnf(errorMsg)
				msg := factories.NewLogMessage(events.LogMessage_ERR, errorMsg, s.appId, "LGR")
				envelope, err := emitter.Wrap(msg, "FIXME")

				if err != nil {
					s.logger.Warnf("Error marshalling message: %v", err)
					continue
				}

				envBytes, err := proto.Marshal(envelope)

				wrappedEnvelope := &envelopewrapper.WrappedEnvelope{
					Envelope:      envelope,
					EnvelopeBytes: envBytes,
				}

				if err == nil {
					s.errorChannel <- wrappedEnvelope
				} else {
					s.logger.Warnf("Error marshalling message: %v", err)
				}
				continue
			}
			s.logger.Infof("Syslog Sink %s: successfully connected.", s.drainUrl)
			s.syslogWriter.SetConnected(true)
			numberOfTries = 0
			defer s.syslogWriter.Close()
		}

		s.logger.Debugf("Syslog Sink %s: Waiting for activity\n", s.drainUrl)

		message, ok := <-buffer.GetOutputChannel()
		if !ok {
			s.logger.Debugf("Syslog Sink %s: Closed listener channel detected. Closing.\n", s.drainUrl)
			return
		}

		s.logger.Debugf("Syslog Sink %s: Got %d bytes. Sending data\n", s.drainUrl, message.EnvelopeLength())

		var err error

		if message.Envelope.GetEventType() != events.Envelope_LogMessage {
			s.logger.Debugf("Syslog Sink %s: Skipping non-log message\n", s.drainUrl)
			continue
		}

		logMessage := message.Envelope.GetLogMessage()

		switch logMessage.GetMessageType() {
		case events.LogMessage_OUT:
			_, err = s.syslogWriter.WriteStdout(logMessage.GetMessage(), logMessage.GetSourceType(), logMessage.GetSourceInstance(), *logMessage.Timestamp)
		case events.LogMessage_ERR:
			_, err = s.syslogWriter.WriteStderr(logMessage.GetMessage(), logMessage.GetSourceType(), logMessage.GetSourceInstance(), *logMessage.Timestamp)
		}
		if err != nil {
			s.logger.Debugf("Syslog Sink %s: Error when trying to send data to sink. Backing off. Err: %v\n", s.drainUrl, err)
			numberOfTries++
			s.syslogWriter.SetConnected(false)
		} else {
			s.logger.Debugf("Syslog Sink %s: Successfully sent data\n", s.drainUrl)
			numberOfTries = 0
			atomic.AddUint64(s.sentMessageCount, 1)
			atomic.AddUint64(s.sentByteCount, uint64(message.EnvelopeLength()))
		}
	}
}

func (s *SyslogSink) Disconnect() {
	close(s.disconnectChannel)
}

func (s *SyslogSink) Identifier() string {
	return s.drainUrl
}

func (s *SyslogSink) AppId() string {
	return s.appId
}

func (s *SyslogSink) ShouldReceiveErrors() bool {
	return false
}

func (s *SyslogSink) Emit() instrumentation.Context {
	return instrumentation.Context{Name: "syslogSink",
		Metrics: []instrumentation.Metric{
			instrumentation.Metric{Name: "sentMessageCount:" + s.appId, Value: atomic.LoadUint64(s.sentMessageCount)},
			instrumentation.Metric{Name: "sentByteCount:" + s.appId, Value: atomic.LoadUint64(s.sentByteCount)},
		},
	}
}
