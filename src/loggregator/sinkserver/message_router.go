package sinkserver

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/sinkserver/metrics"
	"loggregator/sinkserver/sinkmanager"
)

type MessageRouter struct {
	incomingLogChan <-chan *logmessage.Message
	unmarshaller    func([]byte) (*logmessage.Message, error)
	outgoingLogChan chan *logmessage.Message
	SinkManager     *sinkmanager.SinkManager
	Metrics         *metrics.MessageRouterMetrics
	logger          *gosteno.Logger
}

func NewMessageRouter(incomingLogChan <-chan *logmessage.Message, sinkManager *sinkmanager.SinkManager, logger *gosteno.Logger) *MessageRouter {
	return &MessageRouter{
		incomingLogChan: incomingLogChan,
		outgoingLogChan: make(chan *logmessage.Message),
		SinkManager:     sinkManager,
		Metrics:         &metrics.MessageRouterMetrics{},
		logger:          logger,
	}
}

func (r *MessageRouter) Start() {

	for message := range r.incomingLogChan {
		r.logger.Debugf("MessageRouter:outgoingLogChan: Received %d bytes of data from agent listener.", message.GetRawMessageLength())

		r.manageSinks(message)

		r.send(message)
	}
}
func (r *MessageRouter) Stop() {
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
