package sinkserver

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/gosteno"
)

type messageRouter struct {
	SinkManager       *SinkManager
	parsedMessageChan chan *logmessage.Message
	logger            *gosteno.Logger
}

func NewMessageRouter(sinkManager *SinkManager, messageChannelLength int, logger *gosteno.Logger) *messageRouter {
	return &messageRouter{
		SinkManager:       sinkManager,
		parsedMessageChan: make(chan *logmessage.Message, messageChannelLength),
		logger:            logger,
	}
}

func (messageRouter *messageRouter) Start() {
	for message := range messageRouter.parsedMessageChan {
		messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Received %d bytes of data from agent listener.", message.GetRawMessageLength())

		messageRouter.manageSinks(message)

		messageRouter.send(message)
	}
}

func (messageRouter *messageRouter) manageSinks(message *logmessage.Message) {
	logMessage := message.GetLogMessage()
	appId := logMessage.GetAppId()

	if logMessage.GetSourceName() == "App" {
		messageRouter.SinkManager.manageSyslogSinks(appId, logMessage.GetDrainUrls())
	}
	messageRouter.SinkManager.ensureRecentLogsSinkFor(appId)
}

func (messageRouter *messageRouter) send(message *logmessage.Message) {
	logMessage := message.GetLogMessage()
	appId := logMessage.GetAppId()

	messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Searching for sinks with appId [%s].", appId)
	messageRouter.SinkManager.SendTo(appId, message)
	messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Done sending message.")
}
