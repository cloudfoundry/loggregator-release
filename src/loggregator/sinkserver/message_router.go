package sinkserver

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/sinkserver/metrics"
)

type MessageRouter struct {
	unmarshaller func([]byte) (*logmessage.Message, error)
	SinkManager  sinkManager
	Metrics      *metrics.MessageRouterMetrics
	logger       *gosteno.Logger
	done         chan struct{}
}

type sinkManager interface{
	SendTo(string, *logmessage.Message)
	ManageSyslogSinks(string,[]string)
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
	for {
		select {
		case <-r.done:
			return
		case message, ok := <-incomingLogChan:
			if !ok {
				return
			}
			r.logger.Debugf("MessageRouter:outgoingLogChan: Received %d bytes of data from agent listener.", message.GetRawMessageLength())
			r.manageSinks(message)
			r.send(message)
		}
	}
}

func (r *MessageRouter) Stop() {
	close(r.done)
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
