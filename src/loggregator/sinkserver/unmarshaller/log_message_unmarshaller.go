package unmarshaller

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"sync"
)

type LogMessageUnmarshaller struct {
	secret           string
	incomingBytes    <-chan []byte
	outgoingMessages chan *logmessage.Message
	sync.WaitGroup
}

func NewLogMessageUnmarshaller(secret string, incomingBytes <-chan []byte) (*LogMessageUnmarshaller, <-chan *logmessage.Message) {
	outChan := make(chan *logmessage.Message)
	return &LogMessageUnmarshaller{secret: secret, incomingBytes: incomingBytes, outgoingMessages: outChan}, outChan
}

func (l *LogMessageUnmarshaller) Start(errorChan chan<- error) {
	defer close(l.outgoingMessages)
	cfcomponent.Logger.Debug("LogMessageUnmarshaller: Starting")
	for messageBytes := range l.incomingBytes {
		cfcomponent.Logger.Debug("LogMessageUnmarshaller: Parsing a message")
		message, err := logmessage.ParseEnvelope(messageBytes, l.secret)
		if err != nil {
			cfcomponent.Logger.Debugf("LogMessageUnmarshaller: Error while parsing a message %s", err)
			errorChan <- err
			continue
		}
		l.outgoingMessages <- message
		cfcomponent.Logger.Debug("LogMessageUnmarshaller: Sent a message")
	}
}
