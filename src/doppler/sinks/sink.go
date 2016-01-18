package sinks

import (
	"truncatingbuffer"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

type Sink interface {
	AppID() string
	Run(<-chan *events.Envelope)
	Identifier() string
	ShouldReceiveErrors() bool
}

type Metric struct {
	Name  string
	Value int64
}

func RunTruncatingBuffer(inputChan <-chan *events.Envelope, bufferSize uint, context truncatingbuffer.BufferContext, logger *gosteno.Logger, stopChannel chan struct{}) *truncatingbuffer.TruncatingBuffer {
	b := truncatingbuffer.NewTruncatingBuffer(inputChan, bufferSize, context, logger, stopChannel)
	go b.Run()
	return b
}
