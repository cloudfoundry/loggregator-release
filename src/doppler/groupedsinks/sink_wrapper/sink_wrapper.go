package sink_wrapper

import (
	"doppler/sinks"

	"github.com/cloudfoundry/sonde-go/events"
)

type SinkWrapper struct {
	InputChan chan<- *events.Envelope
	Sink      sinks.Sink
}
