package sinks

import (
	"doppler/truncatingbuffer"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
)

type Sink interface {
	StreamId() string
	Run(<-chan *events.Envelope)
	Identifier() string
	ShouldReceiveErrors() bool
	GetInstrumentationMetric() Metric
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
	Tags  map[string]interface{}
}
