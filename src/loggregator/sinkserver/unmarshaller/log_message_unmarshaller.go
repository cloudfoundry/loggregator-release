package unmarshaller

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
)

type LogMessageUnmarshaller struct {
	secret           string
	incomingBytes    <-chan []byte
	outgoingMessages chan *logmessage.Message
}

func NewLogMessageUnmarshaller(secret string, incomingBytes <-chan []byte) (*LogMessageUnmarshaller, <-chan *logmessage.Message) {
	outChan := make(chan *logmessage.Message)
	return &LogMessageUnmarshaller{secret: secret, incomingBytes: incomingBytes, outgoingMessages: outChan}, outChan
}

func (l *LogMessageUnmarshaller) Start(errorChan chan<- error) {
	defer close(l.outgoingMessages)
	for messageBytes := range l.incomingBytes {
		message, err := logmessage.ParseEnvelope(messageBytes, l.secret)
		if err != nil {
			errorChan <- err
			continue
		}
		l.outgoingMessages <- message
	}
}

func (l *LogMessageUnmarshaller) Stop() {

}
