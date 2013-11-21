package sinks

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/buffer"
	"loggregator/buffer/truncatingbuffer"
)

type Sink interface {
	instrumentation.Instrumentable
	AppId() string
	Run()
	Channel() chan *logmessage.Message
	Identifier() string
	Logger() *gosteno.Logger
	ShouldReceiveErrors() bool
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

func runTruncatingBuffer(sink Sink, bufferSize uint, logger *gosteno.Logger) buffer.MessageBuffer {
	b := truncatingbuffer.NewTruncatingBuffer(sink.Channel(), bufferSize, logger)
	go b.Run()
	return b
}
