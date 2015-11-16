package syslog

import (
	"doppler/sinks/retrystrategy"
	"doppler/sinks/syslogwriter"
	"fmt"
	"sync"
	"time"

	"doppler/sinks"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
	"truncatingbuffer"
)

type SyslogSink struct {
	logger                 *gosteno.Logger
	appId                  string
	drainUrl               string
	sentMessageCount       *uint64
	sentByteCount          *uint64
	messageDrainBufferSize uint
	listenerChannel        chan *events.Envelope
	syslogWriter           syslogwriter.Writer
	handleSendError        func(errorMessage, appId, drainUrl string)
	disconnectChannel      chan struct{}
	dropsondeOrigin        string
	disconnectOnce         sync.Once
}

func NewSyslogSink(appId string, drainUrl string, givenLogger *gosteno.Logger, messageDrainBufferSize uint, syslogWriter syslogwriter.Writer, errorHandler func(string, string, string), dropsondeOrigin string) *SyslogSink {
	givenLogger.Debugf("Syslog Sink %s: Created for appId [%s]", drainUrl, appId)
	return &SyslogSink{
		appId:                  appId,
		drainUrl:               drainUrl,
		logger:                 givenLogger,
		messageDrainBufferSize: messageDrainBufferSize,
		syslogWriter:           syslogWriter,
		handleSendError:        errorHandler,
		disconnectChannel:      make(chan struct{}),
		dropsondeOrigin:        dropsondeOrigin,
	}
}

func (s *SyslogSink) Run(inputChan <-chan *events.Envelope) {
	s.logger.Infof("Syslog Sink %s: Running.", s.drainUrl)
	defer s.logger.Errorf("Syslog Sink %s: Stopped.", s.drainUrl)

	backoffStrategy := retrystrategy.NewExponentialRetryStrategy()

	context := truncatingbuffer.NewLogAllowedContext(s.dropsondeOrigin, s.Identifier())
	buffer := sinks.RunTruncatingBuffer(inputChan, s.messageDrainBufferSize, context, s.logger, s.disconnectChannel)
	timer := time.NewTimer(backoffStrategy(0))
	connected := false
	defer timer.Stop()
	defer s.syslogWriter.Close()

	s.logger.Debugf("Syslog Sink %s: Starting loop. Current backoff: %v", s.drainUrl, backoffStrategy(0))
	for {
		s.logger.Debugf("Syslog Sink %s: Waiting for activity\n", s.drainUrl)

		select {
		case <-s.disconnectChannel:
			return
		case messageEnvelope, ok := <-buffer.GetOutputChannel():
			if !ok {
				s.logger.Debugf("Syslog Sink %s: Closed listener channel detected. Closing.\n", s.drainUrl)
				return
			}

			numberOfTries := 0
			for {
				for !connected {
					s.logger.Debugf("Syslog Sink %s: Not connected. Trying to connect.", s.drainUrl)
					err := s.syslogWriter.Connect()
					if err == nil {
						s.logger.Infof("Syslog Sink %s: successfully connected.", s.drainUrl)
						connected = true
						break
					}

					sleepDuration := backoffStrategy(numberOfTries)
					errorMsg := fmt.Sprintf("Syslog Sink %s: Error when dialing out. Backing off for %v. Err: %v", s.drainUrl, sleepDuration, err)

					s.handleSendError(errorMsg, s.appId, s.drainUrl)

					timer.Reset(sleepDuration)
					select {
					case <-s.disconnectChannel:
						return
					case <-timer.C:
					}

					numberOfTries++
				}

				err := s.sendLogMessage(messageEnvelope.GetLogMessage())
				if err == nil {
					connected = true
					break
				}

				s.logger.Debugf("Syslog Sink %s: Error when trying to send data to sink. Backing off. Err: %v\n", s.drainUrl, err)
				connected = false
				numberOfTries++
			}
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

func (s *SyslogSink) sendLogMessage(logMessage *events.LogMessage) error {
	_, err := s.syslogWriter.Write(messagePriorityValue(logMessage), logMessage.GetMessage(), logMessage.GetSourceType(), logMessage.GetSourceInstance(), *logMessage.Timestamp)
	return err
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
