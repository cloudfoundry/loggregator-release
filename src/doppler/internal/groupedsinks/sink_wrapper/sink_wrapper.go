package sink_wrapper

import (
	"doppler/internal/sinks"

	"github.com/cloudfoundry/sonde-go/events"
)

type SinkWrapper struct {
	InputChan chan<- *events.Envelope
	Sink      sinks.Sink
}
