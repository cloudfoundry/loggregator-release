package sinkserver

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
)

type messageRouter struct {
	incomingLogChan chan []byte
	unmarshaller    func([]byte) (*logmessage.Message, error)
	outgoingLogChan chan *logmessage.Message
	SinkManager     *SinkManager
	Metrics         *MessageRouterMetrics
	logger          *gosteno.Logger
}

func NewMessageRouter(incomingLogChan chan []byte, unmarshaller func([]byte) (*logmessage.Message, error), sinkManager *SinkManager, messageChannelLength int, logger *gosteno.Logger) *messageRouter {
	return &messageRouter{
		incomingLogChan: incomingLogChan,
		unmarshaller:    unmarshaller,
		outgoingLogChan: make(chan *logmessage.Message, messageChannelLength),
		SinkManager:     sinkManager,
		Metrics:         &MessageRouterMetrics{},
		logger:          logger,
	}
}

func (messageRouter *messageRouter) Start() {
	go messageRouter.listenForLogs()

	for message := range messageRouter.outgoingLogChan {
		messageRouter.logger.Debugf("MessageRouter:outgoingLogChan: Received %d bytes of data from agent listener.", message.GetRawMessageLength())

		messageRouter.manageSinks(message)

		messageRouter.send(message)
	}
}

func (messageRouter *messageRouter) Emit() instrumentation.Context {
	return messageRouter.Metrics.Emit()
}

func (messageRouter *messageRouter) listenForLogs() {
	for envelopedLog := range messageRouter.incomingLogChan {
		message, err := messageRouter.unmarshaller(envelopedLog)
		if err != nil {
			messageRouter.Metrics.UnmarshalErrorsInParseEnvelopes++
			messageRouter.logger.Errorf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, envelopedLog)
			continue
		}

		messageRouter.Metrics.UnmarshalledInParseEnvelopes++

		select {
		case messageRouter.outgoingLogChan <- message:
		default:
			messageRouter.Metrics.DroppedInParseEnvelopes++
			messageRouter.logger.Debug("MessageRouter:listenForLogs(): incomingLogChan full -- dropping message")
		}
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

	messageRouter.logger.Debugf("MessageRouter:outgoingLogChan: Searching for sinks with appId [%s].", appId)
	messageRouter.SinkManager.SendTo(appId, message)
	messageRouter.logger.Debugf("MessageRouter:outgoingLogChan: Done sending message.")
}
