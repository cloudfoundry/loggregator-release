package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/buffer"
	"loggregator/buffer/ringbuffer"
)

type Sink interface {
	instrumentation.Instrumentable
	AppId() string
	Run()
	Channel() chan *logmessage.Message
	Identifier() string
	Logger() *gosteno.Logger
}

func requestClose(sink Sink, sinkCloseChan chan Sink, alreadyRequestedClose *bool) {
	if !(*alreadyRequestedClose) {
		sinkCloseChan <- sink
		*alreadyRequestedClose = true
		sink.Logger().Debugf("Sink %s: Successfully requested listener channel close", sink)
	} else {
		sink.Logger().Debugf("Sink %s: Previously requested close. Doing nothing", sink)
	}
}

func runNewRingBuffer(sink Sink, bufferSize uint, logger *gosteno.Logger) buffer.MessageBuffer {
	outMessageChan := make(chan *logmessage.Message, bufferSize)
	ringBufferChannel := ringbuffer.NewRingBuffer(sink.Channel(), outMessageChan, logger)
	go ringBufferChannel.Run()
	return ringBufferChannel
}
