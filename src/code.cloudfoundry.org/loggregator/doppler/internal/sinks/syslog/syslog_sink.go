package syslog

import (
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/retrystrategy"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/syslogwriter"
	"code.cloudfoundry.org/loggregator/doppler/internal/truncatingbuffer"

	"github.com/cloudfoundry/sonde-go/events"
)

type SyslogSink struct {
	appId                  string
	drainURL               *url.URL
	sentMessageCount       *uint64
	sentByteCount          *uint64
	messageDrainBufferSize uint
	listenerChannel        chan *events.Envelope
	syslogWriter           syslogwriter.Writer
	handleSendError        func(errorMessage, appId string)
	disconnectChannel      chan struct{}
	dropsondeOrigin        string
	disconnectOnce         sync.Once
}

func NewSyslogSink(appId string, drainURL *url.URL, messageDrainBufferSize uint, syslogWriter syslogwriter.Writer, errorHandler func(string, string), dropsondeOrigin string) *SyslogSink {

	syslogSink := &SyslogSink{
		appId:                  appId,
		drainURL:               drainURL,
		messageDrainBufferSize: messageDrainBufferSize,
		syslogWriter:           syslogWriter,
		handleSendError:        errorHandler,
		disconnectChannel:      make(chan struct{}),
		dropsondeOrigin:        dropsondeOrigin,
	}

	log.Printf("Syslog Sink %s: Created for appId [%s]", syslogSink.Identifier(), appId)
	return syslogSink
}

func (s *SyslogSink) Run(inputChan <-chan *events.Envelope) {
	syslogIdentifier := s.Identifier()
	log.Printf("Syslog Sink %s: Running.", syslogIdentifier)
	defer log.Printf("Syslog Sink %s: Stopped.", syslogIdentifier)

	backoffStrategy := retrystrategy.Exponential()

	context := truncatingbuffer.NewLogAllowedContext(s.dropsondeOrigin, syslogIdentifier)
	buffer := sinks.RunTruncatingBuffer(inputChan, s.messageDrainBufferSize, context, s.disconnectChannel)
	timer := time.NewTimer(backoffStrategy(0))
	connected := false
	defer timer.Stop()
	defer s.syslogWriter.Close()

	log.Printf("Syslog Sink %s: Starting loop. Current backoff: %v", syslogIdentifier, backoffStrategy(0))
	for {
		select {
		case <-s.disconnectChannel:
			return
		case messageEnvelope, ok := <-buffer.GetOutputChannel():
			if !ok {
				log.Printf("Syslog Sink %s: Closed listener channel detected. Closing.\n", syslogIdentifier)
				return
			}

			numberOfTries := 0
			for {
				for !connected {
					err := s.syslogWriter.Connect()
					if err == nil {
						log.Printf("Syslog Sink %s: successfully connected.", syslogIdentifier)
						connected = true
						break
					}

					sleepDuration := backoffStrategy(numberOfTries)
					errorMsg := fmt.Sprintf("Syslog Sink %s: Error when dialing out. Backing off for %v. Err: %v", syslogIdentifier, sleepDuration, err)

					s.handleSendError(errorMsg, s.appId)

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
	if s.drainURL.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s%s", s.drainURL.Scheme, s.drainURL.Host, s.drainURL.Path)
}

func (s *SyslogSink) AppID() string {
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
