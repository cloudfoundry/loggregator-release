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
	var count int

	for {
		envelope := incomingLog.Next()
		count++
		if count%1000 == 0 {
			metric.IncCounter("egress",
				metric.WithIncrement(1000),
				metric.WithVersion(2, 0),
			)

			// TODO: To be removed
			metrics.BatchAddCounter("listeners.receivedEnvelopes", 1000)
		}

		r.send(envelope)
	}
}

func (r *MessageRouter) send(envelope *events.Envelope) {
	appId := envelope_extensions.GetAppId(envelope)

	for _, sm := range r.sinkManagers {
		sm.SendTo(appId, envelope)
	}
}
