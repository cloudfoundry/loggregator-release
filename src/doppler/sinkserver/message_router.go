package sinkserver

import (
	"doppler/sinkserver/metrics"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"sync/atomic"
)

type MessageRouter struct {
	SinkManager sinkManager
	Metrics     *metrics.MessageRouterMetrics
	logger      *gosteno.Logger
	done        chan struct{}
}

type sinkManager interface {
	SendTo(string, *events.Envelope)
}

func NewMessageRouter(sinkManager sinkManager, logger *gosteno.Logger) *MessageRouter {
	return &MessageRouter{
		SinkManager: sinkManager,
		Metrics:     &metrics.MessageRouterMetrics{},
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
			atomic.AddUint64(&r.Metrics.ReceivedMessages, 1)
			r.logger.Debug("MessageRouter:MessageReceived")
			if !ok {
				r.logger.Debug("MessageRouter:MessageReceived:NotOkay")
				return
			}
			r.logger.Debugf("MessageRouter:outgoingLogChan: Received %s message from %s at %d.", envelope.GetEventType().String(), envelope.Origin, envelope.Timestamp)
			r.send(envelope)
		}
	}
}

func (r *MessageRouter) Stop() {
	select {
	case <-r.done:
		// already stopped
	default:
		close(r.done)
	}
}

func (r *MessageRouter) Emit() instrumentation.Context {
	return r.Metrics.Emit()
}

func (r *MessageRouter) send(envelope *events.Envelope) {
	appId := dropsonde.GetAppId(envelope)

	r.logger.Debugf("MessageRouter:outgoingLogChan: Searching for sinks with appId [%s].", appId)
	r.SinkManager.SendTo(appId, envelope)
	r.logger.Debugf("MessageRouter:outgoingLogChan: Done sending message.")
}
