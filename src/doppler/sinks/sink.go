package sinks

import (
	"doppler/buffer"
	"doppler/buffer/truncatingbuffer"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
)

type Sink interface {
	AppId() string
	Run(<-chan *events.Envelope)
	Identifier() string
	ShouldReceiveErrors() bool
}

func RunTruncatingBuffer(inputChan <-chan *events.Envelope, bufferSize uint, logger *gosteno.Logger, dropsondeOrigin string) buffer.MessageBuffer {
	b := truncatingbuffer.NewTruncatingBuffer(inputChan, bufferSize, logger, dropsondeOrigin)
	go b.Run()
	return b
}
