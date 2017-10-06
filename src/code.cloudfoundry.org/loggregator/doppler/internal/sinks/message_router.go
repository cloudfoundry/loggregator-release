package sinks

import (
	"log"
	"sync"

	"code.cloudfoundry.org/loggregator/diodes"
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/sonde-go/events"
)

// MessageRouter consumes from a diode and sends envelopes to all recipients
// passed to the NewMessageRouter constructor.
type MessageRouter struct {
	senders  []EnvelopeSender
	done     chan struct{}
	stopOnce sync.Once
}

// EnvelopeSender is the interface the MessageRouter uses to send envelopes
// read from the diode.
type EnvelopeSender interface {
	SendTo(string, *events.Envelope)
}

// NewMessageRouter is the preferred means of constructing a MessageRouter.
func NewMessageRouter(e ...EnvelopeSender) *MessageRouter {
	return &MessageRouter{
		senders: e,
		done:    make(chan struct{}),
	}
}

// Start begins an infinite loop which reads from the diode and sends any
// received envelopes to the MessageRouter's senders.
func (r *MessageRouter) Start(incomingLog *diodes.ManyToOneEnvelope) {
	log.Print("MessageRouter:Starting")

	for {
		envelope := incomingLog.Next()
		appId := envelope_extensions.GetAppId(envelope)

		for _, sm := range r.senders {
			sm.SendTo(appId, envelope)
		}
	}
}
