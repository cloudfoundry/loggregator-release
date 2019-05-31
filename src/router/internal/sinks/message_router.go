package sinks

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"

	"code.cloudfoundry.org/loggregator/diodes"
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
		appId := getAppId(envelope)

		for _, sm := range r.senders {
			sm.SendTo(appId, envelope)
		}
	}
}

const systemAppId = "system"

func getAppId(envelope *events.Envelope) string {
	if envelope.GetEventType() == events.Envelope_LogMessage {
		return envelope.GetLogMessage().GetAppId()
	}

	if envelope.GetEventType() == events.Envelope_ContainerMetric {
		return envelope.GetContainerMetric().GetApplicationId()
	}

	var event hasAppId
	switch envelope.GetEventType() {
	case events.Envelope_HttpStartStop:
		event = envelope.GetHttpStartStop()
	default:
		return systemAppId
	}

	uuid := event.GetApplicationId()
	if uuid != nil {
		return formatUUID(uuid)
	}
	return systemAppId
}

type hasAppId interface {
	GetApplicationId() *events.UUID
}

func formatUUID(uuid *events.UUID) string {
	var uuidBytes [16]byte
	binary.LittleEndian.PutUint64(uuidBytes[:8], uuid.GetLow())
	binary.LittleEndian.PutUint64(uuidBytes[8:], uuid.GetHigh())
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuidBytes[0:4], uuidBytes[4:6], uuidBytes[6:8], uuidBytes[8:10], uuidBytes[10:])
}
