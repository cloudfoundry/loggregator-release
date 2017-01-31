package sinkserver

import (
	"diodes"
	"log"
	"metric"
	"sync"

	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

type MessageRouter struct {
	sinkManagers []sinkManager
	done         chan struct{}
	stopOnce     sync.Once
}

type sinkManager interface {
	SendTo(string, *events.Envelope)
}

func NewMessageRouter(sinkManagers ...sinkManager) *MessageRouter {
	return &MessageRouter{
		sinkManagers: sinkManagers,
		done:         make(chan struct{}),
	}
}

func (r *MessageRouter) Start(incomingLog *diodes.ManyToOneEnvelope) {
	log.Print("MessageRouter:Starting")
	for {
		envelope := incomingLog.Next()
		metric.IncCounter("egress")

		// TODO: To be removed
		metrics.BatchIncrementCounter("httpServer.receivedMessages")

		r.send(envelope)
	}
}

func (r *MessageRouter) send(envelope *events.Envelope) {
	appId := envelope_extensions.GetAppId(envelope)

	for _, sm := range r.sinkManagers {
		sm.SendTo(appId, envelope)
	}
}
