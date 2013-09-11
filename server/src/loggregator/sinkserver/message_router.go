package sinkserver

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/groupedsinks"
	"loggregator/messagestore"
	"loggregator/sinks"
	"sync"
)

type messageRouter struct {
	parsedMessageChan  chan *logmessage.Message
	sinkCloseChan      chan sinks.Sink
	sinkOpenChan       chan sinks.Sink
	activeSinksCounter int
	messageStore       *messagestore.MessageStore
	logger             *gosteno.Logger
	*sync.RWMutex
}

func NewMessageRouter(messageStore *messagestore.MessageStore, logger *gosteno.Logger) *messageRouter {
	sinkCloseChan := make(chan sinks.Sink, 20)
	sinkOpenChan := make(chan sinks.Sink, 20)
	messageChannel := make(chan *logmessage.Message, 2048)

	return &messageRouter{
		logger:            logger,
		parsedMessageChan: messageChannel,
		sinkCloseChan:     sinkCloseChan,
		sinkOpenChan:      sinkOpenChan,
		messageStore:      messageStore,
		RWMutex:           &sync.RWMutex{}}
}

func (messageRouter *messageRouter) Start() {
	activeSinks := groupedsinks.NewGroupedSinks()

	for {
		select {
		case s := <-messageRouter.sinkOpenChan:
			messageRouter.registerSink(s, activeSinks)
		case s := <-messageRouter.sinkCloseChan:
			messageRouter.unregisterSink(s, activeSinks)
		case receivedMessage := <-messageRouter.parsedMessageChan:
			messageRouter.logger.Debugf("MessageRouter: Received %d bytes of data from agent listener.", receivedMessage.GetRawMessageLength())

			//drain management
			appId := receivedMessage.GetLogMessage().GetAppId()
			if receivedMessage.GetLogMessage().GetSourceType() == logmessage.LogMessage_WARDEN_CONTAINER {
				messageRouter.manageDrains(activeSinks, appId, receivedMessage.GetLogMessage().GetDrainUrls())
			}

			//dump management
			messageRouter.messageStore.Add(receivedMessage, appId)

			//send to drains and sinks
			messageRouter.logger.Debugf("MessageRouter: Searching for sinks with appId [%s].", appId)
			for _, s := range activeSinks.For(appId) {
				messageRouter.logger.Debugf("MessageRouter: Sending Message to channel %v for sinks targeting [%s].", s, appId)
				s.Channel() <- receivedMessage
			}
			messageRouter.logger.Debugf("MessageRouter: Done sending message to tail clients.")
		}
	}
}

func (messageRouter *messageRouter) manageDrains(activeSinks *groupedsinks.GroupedSinks, appId string, drainUrls []string) {
	//delete all drains for app
	if len(drainUrls) == 0 {
		for _, sink := range activeSinks.DrainsFor(appId) {
			messageRouter.unregisterSink(sink, activeSinks)
		}
		return
	}

	//delete all drains that were not sent
	for _, sink := range activeSinks.DrainsFor(appId) {
		if contains(sink.Identifier(), drainUrls) {
			continue
		}
		messageRouter.unregisterSink(sink, activeSinks)
	}

	//add all drains that didn't exist
	for _, drainUrl := range drainUrls {
		if activeSinks.DrainFor(appId, drainUrl) == nil {
			s := sinks.NewSyslogSink(appId, drainUrl, messageRouter.logger)
			go s.Run(messageRouter.sinkCloseChan)
			messageRouter.registerSink(s, activeSinks)
		}
	}
}

func (messageRouter *messageRouter) registerSink(s sinks.Sink, activeSinks *groupedsinks.GroupedSinks) {
	messageRouter.Lock()
	defer messageRouter.Unlock()

	activeSinks.Register(s, s.AppId())
	messageRouter.activeSinksCounter++
	messageRouter.logger.Infof("MessageRouter: Sink with channel %v requested. Opened it.", s.Channel())
}

func (messageRouter *messageRouter) unregisterSink(s sinks.Sink, activeSinks *groupedsinks.GroupedSinks) {
	messageRouter.Lock()
	defer messageRouter.Unlock()

	activeSinks.Delete(s)
	close(s.Channel())
	messageRouter.activeSinksCounter--
	messageRouter.logger.Infof("MessageRouter: Sink with channel %v requested closing. Closed it.", s.Channel())
}

func (messageRouter *messageRouter) metrics() []instrumentation.Metric {
	messageRouter.RLock()
	defer messageRouter.RUnlock()

	return []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfSinks", Value: messageRouter.activeSinksCounter},
	}
}

func (messageRouter *messageRouter) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "messageRouter",
		Metrics: messageRouter.metrics(),
	}
}
