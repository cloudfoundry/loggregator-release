package syslog

import (
	"doppler/sinks"
	"doppler/sinks/retrystrategy"
	"doppler/sinks/syslogwriter"
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"sync"
	"time"
)

const (
	dial_error_debug_string = "Syslog Sink %s: Error when dialing out. Backing off for %v. Err: %v"
	dialing_debug_string    = "Syslog Sink %s: Not connected. Trying to connect."
	starting_loop_debug     = "Syslog Sink %s: Starting loop. Current backoff: %v"
)

type SyslogSink struct {
	*gosteno.Logger
	appId             string
	drainUrl          string
	sentMessageCount  *uint64
	sentByteCount     *uint64
	listenerChannel   chan *events.Envelope
	syslogWriter      syslogwriter.Writer
	handleSendError   func(errorMessage, appId, drainUrl string)
	disconnectChannel chan struct{}
	dropsondeOrigin   string
	disconnectOnce    sync.Once
}

func NewSyslogSink(appId string, drainUrl string, givenLogger *gosteno.Logger, syslogWriter syslogwriter.Writer, errorHandler func(string, string, string), dropsondeOrigin string) sinks.Sink {
	givenLogger.Debugf("Syslog Sink %s: Created for appId [%s]", drainUrl, appId)
	return &SyslogSink{
		appId:             appId,
		drainUrl:          drainUrl,
		Logger:            givenLogger,
		syslogWriter:      syslogWriter,
		handleSendError:   errorHandler,
		disconnectChannel: make(chan struct{}),
		dropsondeOrigin:   dropsondeOrigin,
	}
}

func (s *SyslogSink) Run(inputChan <-chan *events.Envelope) {
	s.Infof("Syslog Sink %s: Running.", s.drainUrl)
	defer s.Errorf("Syslog Sink %s: Stopped.", s.drainUrl)

	backoffStrategy := retrystrategy.NewExponentialRetryStrategy()
	numberOfTries := 0

	buffer := sinks.RunTruncatingBuffer(inputChan, 100, s.Logger, s.dropsondeOrigin)
	timer := time.NewTimer(backoffStrategy(numberOfTries))
	connected := false
	defer timer.Stop()
	defer s.syslogWriter.Close()
	for {
		s.Debugf(starting_loop_debug, s.drainUrl, backoffStrategy(numberOfTries))
		timer.Reset(backoffStrategy(numberOfTries))
		select {
		case <-s.disconnectChannel:
			return
		case <-timer.C:
		}

		if !connected {
			s.Debugf(dialing_debug_string, s.drainUrl)
			err := s.syslogWriter.Connect()
			if err != nil {
				numberOfTries++
				errorMsg := fmt.Sprintf(dial_error_debug_string, s.drainUrl, backoffStrategy(numberOfTries), err)

				s.handleSendError(errorMsg, s.appId, s.drainUrl)
				continue
			}

			s.Infof("Syslog Sink %s: successfully connected.", s.drainUrl)
			connected = true
		}

		s.Debugf("Syslog Sink %s: Waiting for activity\n", s.drainUrl)

		messageEnvelope, ok := <-buffer.GetOutputChannel()
		if !ok {
			s.Debugf("Syslog Sink %s: Closed listener channel detected. Closing.\n", s.drainUrl)
			return
		}

		s.Debugf("Syslog Sink:Run: Received %s message from %s at %d. Sending data.", messageEnvelope.GetEventType().String(), messageEnvelope.GetOrigin(), messageEnvelope.Timestamp)

		_, keepMsg := envelopeTypeWhitelist[messageEnvelope.GetEventType()]
		if !keepMsg {
			s.Debugf("Syslog Sink %s: Skipping non-log message (type %s)", s.drainUrl, messageEnvelope.GetEventType().String())
			continue
		}
		connected = s.sendMessage(messageEnvelope)
		if connected {
			numberOfTries = 0
		} else {
			numberOfTries++
		}
	}
}

func (s *SyslogSink) Disconnect() {
	s.disconnectOnce.Do(func() { close(s.disconnectChannel) })
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

func (s *SyslogSink) sendMessage(messageEnvelope *events.Envelope) bool {
	logMessage := messageEnvelope.GetLogMessage()

	_, err := s.syslogWriter.Write(messagePriorityValue(logMessage), logMessage.GetMessage(), logMessage.GetSourceType(), logMessage.GetSourceInstance(), *logMessage.Timestamp)

	if err != nil {
		s.Debugf("Syslog Sink %s: Error when trying to send data to sink. Backing off. Err: %v\n", s.drainUrl, err)
		return false
	} else {
		s.Debugf("Syslog Sink %s: Successfully sent data\n", s.drainUrl)
		return true
	}
}

var envelopeTypeWhitelist = map[events.Envelope_EventType]struct{}{
	events.Envelope_LogMessage: struct{}{},
}

func messagePriorityValue(msg *events.LogMessage) int {
	switch msg.GetMessageType() {
	case events.LogMessage_OUT:
		return 14
	case events.LogMessage_ERR:
		return 11
	default:
		return -1
	}
}
