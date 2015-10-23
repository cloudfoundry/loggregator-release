package sinkserver

import (
	"sync"

	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/sonde-go/events"
)

type MessageRouter struct {
	sinkManager sinkManager
	logger      *gosteno.Logger
	done        chan struct{}
	stopOnce    sync.Once
}

type sinkManager interface {
	SendTo(string, *events.Envelope)
}

func NewMessageRouter(sinkManager sinkManager, logger *gosteno.Logger) *MessageRouter {
	return &MessageRouter{
		sinkManager: sinkManager,
		logger:      logger,
		done:        make(chan struct{}),
	}
}

func (r *MessageRouter) Start(incomingLogChan <-chan *events.Envelope) {
	r.logger.Debug("MessageRouter:Starting")
	for {
		select {
		case <-r.done:
			r.logger.Debug("MessageRouter:MessageReceived:Done")
			return
		case envelope, ok := <-incomingLogChan:
			if !ok {
				r.logger.Debug("MessageRouter closed")
				return
			}
			r.logger.Debug("MessageRouter:MessageReceived")
			metrics.BatchIncrementCounter("httpServer.receivedMessages")
			r.logger.Debugf("MessageRouter:outgoingLogChan: Received %s message from %s at %d.", envelope.GetEventType().String(), envelope.GetOrigin(), envelope.Timestamp)
			r.send(envelope)
		}
	}
}

func (r *MessageRouter) Stop() {
	r.stopOnce.Do(func() { close(r.done) })
}

func (r *MessageRouter) send(envelope *events.Envelope) {
	appId := envelope_extensions.GetAppId(envelope)

	r.logger.Debugf("MessageRouter:outgoingLogChan: Searching for sinks with appId [%s].", appId)
	r.sinkManager.SendTo(appId, envelope)
	r.logger.Debugf("MessageRouter:outgoingLogChan: Done sending message.")
}
