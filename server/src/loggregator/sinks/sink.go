package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
)

type Sink interface {
	instrumentation.Instrumentable
	AppId() string
	Run(sinkCloseChan chan Sink)
	Channel() chan *logmessage.Message
	Identifier() string
	Logger() *gosteno.Logger
}

func requestClose(sink Sink, sinkCloseChan chan Sink, alreadyRequestedClose bool) {
	if !alreadyRequestedClose {
		sinkCloseChan <- sink
		alreadyRequestedClose = true
		sink.Logger().Debugf("Sink %s: Successfully requested listener channel close", sink)
	} else {
		sink.Logger().Debugf("Sink %s: Previously requested close. Doing nothing", sink)
	}
}

func newRingBufferChannel(sink Sink) <-chan *logmessage.Message {
	outMessageChan := make(chan *logmessage.Message, 10)
	go RingBufferChannel(sink.Channel(), outMessageChan, sink.Logger())
	return outMessageChan
}
