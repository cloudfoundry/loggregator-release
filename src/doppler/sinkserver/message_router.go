package sinkserver

import (
	"doppler/envelopewrapper"
	"doppler/sinkserver/metrics"
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
	SendTo(string, *envelopewrapper.WrappedEnvelope)
}

func NewMessageRouter(sinkManager sinkManager, logger *gosteno.Logger) *MessageRouter {
	return &MessageRouter{
		SinkManager: sinkManager,
		Metrics:     &metrics.MessageRouterMetrics{},
		logger:      logger,
		done:        make(chan struct{}),
	}
}

func (r *MessageRouter) Start(incomingLogChan <-chan *envelopewrapper.WrappedEnvelope) {
	r.logger.Debug("MessageRouter:Starting")
	for {
		select {
		case <-r.done:
			r.logger.Debug("MessageRouter:MessageReceived:Done")
			return
		case wrappedEnvelope, ok := <-incomingLogChan:
			atomic.AddUint64(&r.Metrics.ReceivedMessages, 1)
			r.logger.Debug("MessageRouter:MessageReceived")
			if !ok {
				r.logger.Debug("MessageRouter:MessageReceived:NotOkay")
				return
			}
			r.logger.Debugf("MessageRouter:outgoingLogChan: Received %d bytes of data from agent listener.", wrappedEnvelope.EnvelopeLength())
			r.send(wrappedEnvelope)
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

func (r *MessageRouter) send(wrappedEnvelope *envelopewrapper.WrappedEnvelope) {
	appId := wrappedEnvelope.Envelope.GetAppId()

	r.logger.Debugf("MessageRouter:outgoingLogChan: Searching for sinks with appId [%s].", appId)
	r.SinkManager.SendTo(appId, wrappedEnvelope)
	r.logger.Debugf("MessageRouter:outgoingLogChan: Done sending message.")
}
