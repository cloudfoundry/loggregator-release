package sinks

import (
	"doppler/truncatingbuffer"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

type Sink interface {
	StreamId() string
	Run(<-chan *events.Envelope)
	Identifier() string
	ShouldReceiveErrors() bool
	UpdateDroppedMessageCount(int64)
}

func RunTruncatingBuffer(inputChan <-chan *events.Envelope, bufferSize uint, logger *gosteno.Logger, dropsondeOrigin string) *truncatingbuffer.TruncatingBuffer {
	b := truncatingbuffer.NewTruncatingBuffer(inputChan, bufferSize, logger, dropsondeOrigin)
	go b.Run()
	return b
}

type Metric struct {
	Name  string
	Value int64
}
