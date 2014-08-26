package sinks

import (
	"doppler/buffer"
	"doppler/buffer/truncatingbuffer"
	"doppler/envelopewrapper"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
)

type Sink interface {
	instrumentation.Instrumentable
	AppId() string
	Run(<-chan *envelopewrapper.WrappedEnvelope)
	Identifier() string
	ShouldReceiveErrors() bool
}

func RunTruncatingBuffer(inputChan <-chan *envelopewrapper.WrappedEnvelope, bufferSize uint, logger *gosteno.Logger, dropsondeOrigin string) buffer.MessageBuffer {
	b := truncatingbuffer.NewTruncatingBuffer(inputChan, bufferSize, logger, dropsondeOrigin)
	go b.Run()
	return b
}
