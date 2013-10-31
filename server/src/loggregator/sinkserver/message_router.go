package sinkserver

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/groupedsinks"
	"loggregator/sinks"
	"net/url"
	"sync"
)

type messageRouter struct {
	dumpBuffer                  uint
	parsedMessageChan           chan *logmessage.Message
	sinkCloseChan               chan sinks.Sink
	sinkOpenChan                chan sinks.Sink
	dumpReceiverChan            chan dumpReceiver
	activeDumpSinksCounter      int
	activeWebsocketSinksCounter int
	activeSyslogSinksCounter    int
	logger                      *gosteno.Logger
	*sync.RWMutex
}

func NewMessageRouter(maxRetainedLogMessages uint, logger *gosteno.Logger) *messageRouter {
	sinkCloseChan := make(chan sinks.Sink, 20)
	sinkOpenChan := make(chan sinks.Sink, 20)
	dumpReceiverChan := make(chan dumpReceiver)
	messageChannel := make(chan *logmessage.Message, 2048)

	return &messageRouter{
		logger:            logger,
		parsedMessageChan: messageChannel,
		sinkCloseChan:     sinkCloseChan,
		sinkOpenChan:      sinkOpenChan,
		dumpReceiverChan:  dumpReceiverChan,
		dumpBuffer:        maxRetainedLogMessages,
		RWMutex:           &sync.RWMutex{}}
}

func (messageRouter *messageRouter) Start() {
	activeSinks := groupedsinks.NewGroupedSinks()

	for {
		select {
		case dr := <-messageRouter.dumpReceiverChan:
			if sink := activeSinks.DumpFor(dr.appId); sink != nil {
				sink.Dump(dr.outputChannel)
			}
		case s := <-messageRouter.sinkOpenChan:
			messageRouter.registerSink(s, activeSinks)
		case s := <-messageRouter.sinkCloseChan:
			messageRouter.unregisterSink(s, activeSinks)
		case receivedMessage := <-messageRouter.parsedMessageChan:
			messageRouter.logger.Debugf("MessageRouter: Received %d bytes of data from agent listener.", receivedMessage.GetRawMessageLength())

			//drain management
			appId := receivedMessage.GetLogMessage().GetAppId()
			messageRouter.manageDrains(activeSinks, appId, receivedMessage.GetLogMessage().GetDrainUrls(), receivedMessage.GetLogMessage().GetSourceType())
			messageRouter.manageDumps(activeSinks, appId)

			//send to drains and sinks
			messageRouter.logger.Debugf("MessageRouter: Searching for sinks with appId [%s].", appId)
			for _, s := range activeSinks.For(appId) {
				messageRouter.logger.Debugf("MessageRouter: Sending Message to channel %v for sinks targeting [%s].", s.Identifier(), appId)
				s.Channel() <- receivedMessage
			}
			messageRouter.logger.Debugf("MessageRouter: Done sending message to tail clients.")
		}
	}
}

func (messageRouter *messageRouter) registerDumpChan(appId string) <-chan *logmessage.Message {
	dumpChan := make(chan *logmessage.Message, messageRouter.dumpBuffer)
	dr := dumpReceiver{appId: appId, outputChannel: dumpChan}
	messageRouter.dumpReceiverChan <- dr
	return dumpChan
}

func (messageRouter *messageRouter) registerSink(s sinks.Sink, activeSinks *groupedsinks.GroupedSinks) bool {
	messageRouter.Lock()
	defer messageRouter.Unlock()

	ok := activeSinks.Register(s)
	switch s.(type) {
	case *sinks.DumpSink:
		messageRouter.activeDumpSinksCounter++
	case *sinks.SyslogSink:
		messageRouter.activeSyslogSinksCounter++
	case *sinks.WebsocketSink:
		messageRouter.activeWebsocketSinksCounter++
	}
	messageRouter.logger.Infof("MessageRouter: Sink with channel %v requested. Opened it.", s.Channel())
	return ok
}

func (messageRouter *messageRouter) unregisterSink(s sinks.Sink, activeSinks *groupedsinks.GroupedSinks) {
	messageRouter.Lock()
	defer messageRouter.Unlock()

	activeSinks.Delete(s)
	close(s.Channel())
	switch s.(type) {
	case *sinks.DumpSink:
		messageRouter.activeDumpSinksCounter--
	case *sinks.SyslogSink:
		messageRouter.activeSyslogSinksCounter--
	case *sinks.WebsocketSink:
		messageRouter.activeWebsocketSinksCounter--
	}
	messageRouter.logger.Infof("MessageRouter: Sink with channel %v requested closing. Closed it.", s.Channel())
}

func (messageRouter *messageRouter) manageDumps(activeSinks *groupedsinks.GroupedSinks, appId string) {
	if activeSinks.DumpFor(appId) == nil {
		s := sinks.NewDumpSink(appId, messageRouter.dumpBuffer, messageRouter.logger)
		ok := messageRouter.registerSink(s, activeSinks)
		if ok {
			go s.Run(messageRouter.sinkCloseChan)
		}
	}
}

func (messageRouter *messageRouter) manageDrains(activeSinks *groupedsinks.GroupedSinks, appId string, drainUrls []string, sourceType string) {
	if sourceType != "APP" {
		return
	}
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
			dl, err := url.Parse(drainUrl)
			if err != nil {
				messageRouter.logger.Warnf("MessageRouter: Error when trying to parse syslog url %v. Requesting close. Err: %v", drainUrl, err)
				continue
			}
			sysLogger := sinks.NewSyslogWriter("tcp", dl.Host, appId)
			s := sinks.NewSyslogSink(appId, drainUrl, messageRouter.logger, sysLogger)
			ok := messageRouter.registerSink(s, activeSinks)
			if ok {
				go s.Run(messageRouter.sinkCloseChan)
			}
		}
	}
}

func (messageRouter *messageRouter) metrics() []instrumentation.Metric {
	messageRouter.RLock()
	defer messageRouter.RUnlock()

	return []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfDumpSinks", Value: messageRouter.activeDumpSinksCounter},
		instrumentation.Metric{Name: "numberOfSyslogSinks", Value: messageRouter.activeSyslogSinksCounter},
		instrumentation.Metric{Name: "numberOfWebsocketSinks", Value: messageRouter.activeWebsocketSinksCounter},
	}
}

func (messageRouter *messageRouter) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "messageRouter",
		Metrics: messageRouter.metrics(),
	}
}
