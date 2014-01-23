package sinkserver

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/groupedsinks"
	"loggregator/iprange"
	"loggregator/sinks"
	"net/url"
	"sync"
)

type messageRouter struct {
	dumpBufferSize              int
	parsedMessageChan           chan *logmessage.Message
	sinkCloseChan               chan sinks.Sink
	sinkOpenChan                chan sinks.Sink
	dumpReceiverChan            chan dumpReceiver
	activeDumpSinksCounter      int
	activeWebsocketSinksCounter int
	activeSyslogSinksCounter    int
	logger                      *gosteno.Logger
	errorChannel                chan *logmessage.Message
	skipCertVerify              bool
	blackListIPs                []iprange.IPRange
	blacklistedURLS             []string
	*sync.RWMutex
}

func NewMessageRouter(maxRetainedLogMessages int, skipCertVerify bool, blackListIPs []iprange.IPRange, logger *gosteno.Logger) *messageRouter {
	sinkCloseChan := make(chan sinks.Sink, 20)
	sinkOpenChan := make(chan sinks.Sink, 20)
	dumpReceiverChan := make(chan dumpReceiver, 10)
	messageChannel := make(chan *logmessage.Message, 2048)

	return &messageRouter{
		logger:            logger,
		parsedMessageChan: messageChannel,
		sinkCloseChan:     sinkCloseChan,
		sinkOpenChan:      sinkOpenChan,
		dumpReceiverChan:  dumpReceiverChan,
		dumpBufferSize:    maxRetainedLogMessages,
		RWMutex:           &sync.RWMutex{},
		errorChannel:      make(chan *logmessage.Message, 100),
		blackListIPs:      blackListIPs,
		skipCertVerify:    skipCertVerify}
}

func (messageRouter *messageRouter) Start() {
	activeSinks := groupedsinks.NewGroupedSinks()

	go func() {
		for {
			select {
			case dr := <-messageRouter.dumpReceiverChan:
				go func() {
					if sink := activeSinks.DumpFor(dr.appId); sink != nil {
						sink.Dump(dr.outputChannel)
					} else {
						messageRouter.logger.Debugf("MessageRouter:DumpReceiverChan: No dump exists for appId [%s].", dr.appId)
						close(dr.outputChannel)
					}
				}()
			case s := <-messageRouter.sinkOpenChan:
				messageRouter.registerSink(s, activeSinks)
			case s := <-messageRouter.sinkCloseChan:
				messageRouter.unregisterSink(s, activeSinks)
			case receivedMessage := <-messageRouter.parsedMessageChan:
				messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Received %d bytes of data from agent listener.", receivedMessage.GetRawMessageLength())

				//drain management
				appId := receivedMessage.GetLogMessage().GetAppId()
				messageRouter.manageDrains(activeSinks, appId, receivedMessage.GetLogMessage().GetDrainUrls(), receivedMessage.GetLogMessage().GetSourceName())
				messageRouter.manageDumps(activeSinks, appId)

				//send to drains and sinks
				messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Searching for sinks with appId [%s].", appId)
				for _, s := range activeSinks.For(appId) {
					messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Sending Message to channel %v for sinks targeting [%s].", s.Identifier(), appId)
					s.Channel() <- receivedMessage
				}
				messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Done sending message.")
			}
		}
	}()
	for {
		select {
		case receivedMessage := <-messageRouter.errorChannel:
			appId := receivedMessage.GetLogMessage().GetAppId()

			messageRouter.logger.Debugf("MessageRouter:ErrorChannel: Searching for sinks with appId [%s].", appId)
			for _, s := range activeSinks.For(appId) {
				if s.ShouldReceiveErrors() {
					messageRouter.logger.Debugf("MessageRouter:ErrorChannel: Sending Message to channel %v for sinks targeting [%s].", s.Identifier(), appId)
					s.Channel() <- receivedMessage
				}
			}
			messageRouter.logger.Debugf("MessageRouter:ErrorChannel: Done sending error message.")
		}
	}
}

func (messageRouter *messageRouter) getDumpChanFor(appId string) <-chan *logmessage.Message {
	dumpChan := make(chan *logmessage.Message, messageRouter.dumpBufferSize)
	dr := dumpReceiver{appId: appId, outputChannel: dumpChan}
	messageRouter.dumpReceiverChan <- dr
	return dumpChan
}

func (messageRouter *messageRouter) registerSink(s sinks.Sink, activeSinks *groupedsinks.GroupedSinks) bool {
	messageRouter.Lock()
	defer messageRouter.Unlock()

	ok := activeSinks.Register(s)
	if !ok {
		return false
	}

	switch s.(type) {
	case *sinks.DumpSink:
		messageRouter.activeDumpSinksCounter++
	case *sinks.SyslogSink:
		messageRouter.activeSyslogSinksCounter++
	case *sinks.WebsocketSink:
		messageRouter.activeWebsocketSinksCounter++
	}
	messageRouter.logger.Infof("MessageRouter: Sink with channel %v requested. Opened it.", s.Channel())
	return true
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
		syslogSink, _ := s.(*sinks.SyslogSink)
		syslogSink.Disconnect()
		messageRouter.activeSyslogSinksCounter--
	case *sinks.WebsocketSink:
		messageRouter.activeWebsocketSinksCounter--
	}
	messageRouter.logger.Infof("MessageRouter: Sink with channel %v and identifier %s requested closing. Closed it.", s.Channel(), s.Identifier())
}

func (messageRouter *messageRouter) manageDumps(activeSinks *groupedsinks.GroupedSinks, appId string) {
	if activeSinks.DumpFor(appId) != nil {
		return
	}

	s := sinks.NewDumpSink(appId, messageRouter.dumpBufferSize, messageRouter.logger)

	if messageRouter.registerSink(s, activeSinks) {
		go s.Run()
	}
}

func (messageRouter *messageRouter) manageDrains(activeSinks *groupedsinks.GroupedSinks, appId string, drainUrls []string, sourceName string) {
	if sourceName != "App" {
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
		if activeSinks.DrainFor(appId, drainUrl) == nil && !messageRouter.urlIsBlackListed(drainUrl) {
			dl, err := url.Parse(drainUrl)
			if err != nil {
				messageRouter.blacklistedURLS = append(messageRouter.blacklistedURLS, drainUrl)
				errorMessage := fmt.Sprintf("MessageRouter: Error when trying to parse syslog url %v. Requesting close. Err: %v", drainUrl, err)
				messageRouter.sendLoggregatorErrorMessage(errorMessage, appId)
				continue
			}
			ipNotBlacklisted, err := iprange.IpOutsideOfRanges(*dl, messageRouter.blackListIPs)
			if err != nil {
				messageRouter.blacklistedURLS = append(messageRouter.blacklistedURLS, drainUrl)
				errorMessage := fmt.Sprintf("MessageRouter: Error when trying to check syslog url %v against blacklist ip ranges. Requesting close. Err: %v", drainUrl, err)
				messageRouter.sendLoggregatorErrorMessage(errorMessage, appId)
				continue
			}
			if ipNotBlacklisted {
				sysLogger := sinks.NewSyslogWriter(dl.Scheme, dl.Host, appId, messageRouter.skipCertVerify)
				s := sinks.NewSyslogSink(appId, drainUrl, messageRouter.logger, sysLogger, messageRouter.errorChannel)
				ok := messageRouter.registerSink(s, activeSinks)
				if ok {
					go s.Run()
				}
			} else {
				messageRouter.blacklistedURLS = append(messageRouter.blacklistedURLS, drainUrl)
				errorMsg := fmt.Sprintf("MessageRouter: Syslog drain url is blacklisted: %s", drainUrl)
				messageRouter.sendLoggregatorErrorMessage(errorMsg, appId)
			}
		}
	}
}

func (messageRouter *messageRouter) sendLoggregatorErrorMessage(errorMsg, appId string) {
	messageRouter.logger.Warnf(errorMsg)
	logMessage, err := logmessage.GenerateMessage(logmessage.LogMessage_ERR, errorMsg, appId, "LGR")
	if err == nil {
		messageRouter.errorChannel <- logMessage
	} else {
		messageRouter.logger.Warnf("Error marshalling message: %v", err)
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

func (messageRouter *messageRouter) urlIsBlackListed(testUrl string) bool {
	for _, url := range messageRouter.blacklistedURLS {
		if url == testUrl {
			return true
		}
	}
	return false
}
