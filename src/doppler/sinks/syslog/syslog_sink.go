package syslog

import (
	"doppler/sinks"
	"doppler/sinks/retrystrategy"
	"doppler/sinks/syslogwriter"
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"time"
)

type SyslogSink struct {
	logger            *gosteno.Logger
	appId             string
	drainUrl          string
	sentMessageCount  *uint64
	sentByteCount     *uint64
	listenerChannel   chan *events.Envelope
	syslogWriter      syslogwriter.SyslogWriter
	handleSendError   func(errorMessage, appId, drainUrl string)
	disconnectChannel chan struct{}
	dropsondeOrigin   string
}

func NewSyslogSink(appId string, drainUrl string, givenLogger *gosteno.Logger, syslogWriter syslogwriter.SyslogWriter, errorHandler func(string, string, string), dropsondeOrigin string) sinks.Sink {
	givenLogger.Debugf("Syslog Sink %s: Created for appId [%s]", drainUrl, appId)
	return &SyslogSink{
		appId:             appId,
		drainUrl:          drainUrl,
		logger:            givenLogger,
		syslogWriter:      syslogWriter,
		handleSendError:   errorHandler,
		disconnectChannel: make(chan struct{}),
		dropsondeOrigin:   dropsondeOrigin,
	}
}

func (s *SyslogSink) Run(inputChan <-chan *events.Envelope) {
	s.logger.Infof("Syslog Sink %s: Running.", s.drainUrl)
	defer s.logger.Errorf("Syslog Sink %s: Stopped.", s.drainUrl)

	backoffStrategy := retrystrategy.NewExponentialRetryStrategy()
	numberOfTries := 0

	buffer := sinks.RunTruncatingBuffer(inputChan, 100, s.logger, s.dropsondeOrigin)
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
				numberOfTries++
				errorMsg := fmt.Sprintf("Syslog Sink %s: Error when dialing out. Backing off for %v. Err: %v", s.drainUrl, backoffStrategy(numberOfTries), err)

				s.handleSendError(errorMsg, s.appId, s.drainUrl)
				continue
			}

			s.logger.Infof("Syslog Sink %s: successfully connected.", s.drainUrl)
			s.syslogWriter.SetConnected(true)
			numberOfTries = 0
			defer s.syslogWriter.Close()
		}

		s.logger.Debugf("Syslog Sink %s: Waiting for activity\n", s.drainUrl)

		messageEnvelope, ok := <-buffer.GetOutputChannel()
		if !ok {
			s.logger.Debugf("Syslog Sink %s: Closed listener channel detected. Closing.\n", s.drainUrl)
			return
		}

		s.logger.Debugf("SyslogSink:Run: Received %s message from %s at %d. Sending data.", messageEnvelope.GetEventType().String(), messageEnvelope.Origin, messageEnvelope.Timestamp)

		var err error

		_, keepMsg := envelopeTypeWhitelist[messageEnvelope.GetEventType()]
		if !keepMsg {
			s.logger.Debugf("Syslog sink %s: Skipping non-log message (type %s)", s.drainUrl, messageEnvelope.GetEventType().String())
			continue
		}

		logMessage := messageEnvelope.GetLogMessage()

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
		}
	}
}

func (s *SyslogSink) Disconnect() {
	select {
	case <- s.disconnectChannel:
		s.logger.Debugf("SyslogSink.Disconnect: already disconnected from %s.", s.drainUrl)
	default:
		close(s.disconnectChannel)
	}
}

func (s *SyslogSink) Identifier() string {
	return s.drainUrl
}

func (s *SyslogSink) StreamId() string {
	return s.appId
}

func (s *SyslogSink) ShouldReceiveErrors() bool {
	return false
}

var envelopeTypeWhitelist = map[events.Envelope_EventType]struct{}{
	events.Envelope_LogMessage: struct{}{},
}
