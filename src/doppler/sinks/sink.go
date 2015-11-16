package sinks

import (
	"truncatingbuffer"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

type Sink interface {
	StreamId() string
	Run(<-chan *events.Envelope)
	Identifier() string
	ShouldReceiveErrors() bool
}

type Metric struct {
	Name  string
	Value int64
}

func RunTruncatingBuffer(inputChan <-chan *events.Envelope, filterEvent func(events.Envelope_EventType) bool, bufferSize uint, logger *gosteno.Logger, dropsondeOrigin, sinkIdentifier string, stopChannel chan struct{}) *truncatingbuffer.TruncatingBuffer {
	b := truncatingbuffer.NewTruncatingBuffer(inputChan, filterEvent, bufferSize, logger, dropsondeOrigin, sinkIdentifier, stopChannel)
	go b.Run()
	return b
}
