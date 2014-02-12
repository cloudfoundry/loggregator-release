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
	Run(<-chan *logmessage.Message)
	Identifier() string
	ShouldReceiveErrors() bool
}

func RunTruncatingBuffer(inputChan <-chan *logmessage.Message, bufferSize uint, logger *gosteno.Logger) buffer.MessageBuffer {
	b := truncatingbuffer.NewTruncatingBuffer(inputChan, bufferSize, logger)
	go b.Run()
	return b
}
