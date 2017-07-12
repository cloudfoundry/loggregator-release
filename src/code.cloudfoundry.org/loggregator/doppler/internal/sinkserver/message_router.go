package sinkserver

import (
	"log"
	"sync"

	"code.cloudfoundry.org/loggregator/diodes"

	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/sonde-go/events"
)

type MessageRouter struct {
	senders  []EnvelopeSender
	done     chan struct{}
	stopOnce sync.Once
}

type EnvelopeSender interface {
	SendTo(string, *events.Envelope)
}

func NewMessageRouter(e ...EnvelopeSender) *MessageRouter {
	return &MessageRouter{
		senders: e,
		done:    make(chan struct{}),
	}
}

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
