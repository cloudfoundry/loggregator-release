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
	var count int

	for {
		envelope := incomingLog.Next()
		count++
		if count%1000 == 0 {
			// metric:v2 (loggregator.doppler.egress) Number of v1 envelopes
			// read from a diode to be sent to TrafficController consumers.
			metric.IncCounter("egress",
				metric.WithIncrement(1000),
				metric.WithVersion(2, 0),
			)

			// metric:v1 (listeners.receivedEnvelopes) Number of v1 envelopes
			// read from a diode to be sent to TrafficController consumers.
			metrics.BatchAddCounter("listeners.receivedEnvelopes", 1000)
		}

		appId := envelope_extensions.GetAppId(envelope)

		for _, sm := range r.senders {
			sm.SendTo(appId, envelope)
		}
	}
}
