package sinkserver

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/iprange"
)

type messageRouter struct {
	parsedMessageChan chan *logmessage.Message
	logger            *gosteno.Logger
	SinkManager *SinkManager
}

func NewMessageRouter(maxRetainedLogMessages int, skipCertVerify bool, blackListIPs []iprange.IPRange, logger *gosteno.Logger, messageChannelLength int) *messageRouter {
	messageChannel := make(chan *logmessage.Message, messageChannelLength)

	sinkManager := NewSinkManager(maxRetainedLogMessages, skipCertVerify, blackListIPs, logger)
	go sinkManager.Start()

	return &messageRouter{
		parsedMessageChan: messageChannel,
		logger:            logger,
		SinkManager:       sinkManager,
	}
}

func (messageRouter *messageRouter) Start() {
	for receivedMessage := range messageRouter.parsedMessageChan {
		messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Received %d bytes of data from agent listener.", receivedMessage.GetRawMessageLength())

		messageRouter.manageSyslogSinks(receivedMessage)
		messageRouter.manageDumpSinks(receivedMessage)
		messageRouter.sendToActiveSinks(receivedMessage)
	}
}

func (messageRouter *messageRouter) manageSyslogSinks(receivedMessage *logmessage.Message) {
	logMessage := receivedMessage.GetLogMessage()
	appId := logMessage.GetAppId()

	if logMessage.GetSourceName() == "App" {
		messageRouter.SinkManager.manageSyslogSinks(appId, logMessage.GetDrainUrls())
	}
}

func (messageRouter *messageRouter) manageDumpSinks(receivedMessage *logmessage.Message) {
	logMessage := receivedMessage.GetLogMessage()
	appId := logMessage.GetAppId()

	messageRouter.SinkManager.manageDumpSinks(appId)
}

func (messageRouter *messageRouter) sendToActiveSinks(receivedMessage *logmessage.Message) {
	logMessage := receivedMessage.GetLogMessage()
	appId := logMessage.GetAppId()

	messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Searching for sinks with appId [%s].", appId)
	messageRouter.SinkManager.sendToActiveSinks(appId, receivedMessage)
	messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Done sending message.")
}
