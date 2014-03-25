package sinkserver

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/sinkserver/metrics"
	"sync/atomic"
)

type MessageRouter struct {
	unmarshaller func([]byte) (*logmessage.Message, error)
	SinkManager  sinkManager
	Metrics      *metrics.MessageRouterMetrics
	logger       *gosteno.Logger
	done         chan struct{}
}

type sinkManager interface {
	SendTo(string, *logmessage.Message)
	ManageSyslogSinks(string, []string)
}

func NewMessageRouter(sinkManager sinkManager, logger *gosteno.Logger) *MessageRouter {
	return &MessageRouter{
		SinkManager: sinkManager,
		Metrics:     &metrics.MessageRouterMetrics{},
		logger:      logger,
		done:        make(chan struct{}),
	}
}

func (r *MessageRouter) Start(incomingLogChan <-chan *logmessage.Message) {
	r.logger.Debug("MessageRouter:Starting")
	for {
		select {
		case <-r.done:
			r.logger.Debug("MessageRouter:MessageReceived:Done")
			return
		case message, ok := <-incomingLogChan:
			atomic.AddUint64(&r.Metrics.ReceivedMessages, 1)
			r.logger.Debug("MessageRouter:MessageReceived")
			if !ok {
				r.logger.Debug("MessageRouter:MessageReceived:NotOkay")
				return
			}
			r.logger.Debugf("MessageRouter:outgoingLogChan: Received %d bytes of data from agent listener.", message.GetRawMessageLength())
			r.manageSinks(message)
			r.send(message)
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

func (r *MessageRouter) manageSinks(message *logmessage.Message) {
	logMessage := message.GetLogMessage()
	appId := logMessage.GetAppId()

	if logMessage.GetSourceName() == "App" {
		r.SinkManager.ManageSyslogSinks(appId, logMessage.GetDrainUrls())
	}
}

func (r *MessageRouter) send(message *logmessage.Message) {
	logMessage := message.GetLogMessage()
	appId := logMessage.GetAppId()

	r.logger.Debugf("MessageRouter:outgoingLogChan: Searching for sinks with appId [%s].", appId)
	r.SinkManager.SendTo(appId, message)
	r.logger.Debugf("MessageRouter:outgoingLogChan: Done sending message.")
}
