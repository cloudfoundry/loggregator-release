package sinks

import (
	"code.cloudfoundry.org/loggregator/doppler/internal/truncatingbuffer"

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

func RunTruncatingBuffer(inputChan <-chan *events.Envelope, bufferSize uint, context truncatingbuffer.BufferContext, stopChannel chan struct{}) *truncatingbuffer.TruncatingBuffer {
	b := truncatingbuffer.NewTruncatingBuffer(inputChan, bufferSize, context, stopChannel)
	go b.Run()
	return b
}
