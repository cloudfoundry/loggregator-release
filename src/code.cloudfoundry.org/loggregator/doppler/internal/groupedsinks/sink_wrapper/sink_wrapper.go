package sink_wrapper

import (
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"

	"github.com/cloudfoundry/sonde-go/events"
)

type SinkWrapper struct {
	InputChan chan<- *events.Envelope
	Sink      sinks.Sink
}
