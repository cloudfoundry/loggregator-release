package sinkserver

import (
	"errors"
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
	activeDumpSinksCounter      int
	activeWebsocketSinksCounter int
	activeSyslogSinksCounter    int
	logger                      *gosteno.Logger
	errorChannel                chan *logmessage.Message
	skipCertVerify              bool
	blackListIPs                []iprange.IPRange
	blacklistedURLS             []string
	activeSinks                 *groupedsinks.GroupedSinks
	*sync.RWMutex
}

func NewMessageRouter(maxRetainedLogMessages int, skipCertVerify bool, blackListIPs []iprange.IPRange, logger *gosteno.Logger, messageChannelLength int) *messageRouter {
	sinkCloseChan := make(chan sinks.Sink, 20)
	sinkOpenChan := make(chan sinks.Sink, 20)
	messageChannel := make(chan *logmessage.Message, messageChannelLength)

	return &messageRouter{
		logger:            logger,
		parsedMessageChan: messageChannel,
		sinkCloseChan:     sinkCloseChan,
		sinkOpenChan:      sinkOpenChan,
		dumpBufferSize:    maxRetainedLogMessages,
		RWMutex:           &sync.RWMutex{},
		errorChannel:      make(chan *logmessage.Message, 100),
		blackListIPs:      blackListIPs,
		skipCertVerify:    skipCertVerify,
		activeSinks:       groupedsinks.NewGroupedSinks()}
}

func (messageRouter *messageRouter) Start() {

	go func() {
		for {
			select {
			case s := <-messageRouter.sinkOpenChan:
				messageRouter.registerSink(s)
			case s := <-messageRouter.sinkCloseChan:
				messageRouter.unregisterSink(s)
			case receivedMessage := <-messageRouter.parsedMessageChan:
				messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Received %d bytes of data from agent listener.", receivedMessage.GetRawMessageLength())

				//drain management
				appId := receivedMessage.GetLogMessage().GetAppId()
				if receivedMessage.GetLogMessage().GetSourceName() == "App" {
					messageRouter.manageDrains(appId, receivedMessage.GetLogMessage().GetDrainUrls())
				}

				messageRouter.manageDumps(appId)

				//send to drains and sinks
				messageRouter.logger.Debugf("MessageRouter:ParsedMessageChan: Searching for sinks with appId [%s].", appId)
				for _, s := range messageRouter.activeSinks.For(appId) {
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
			for _, s := range messageRouter.activeSinks.For(appId) {
				if s.ShouldReceiveErrors() {
					messageRouter.logger.Debugf("MessageRouter:ErrorChannel: Sending Message to channel %v for sinks targeting [%s].", s.Identifier(), appId)
					s.Channel() <- receivedMessage
				}
			}
			messageRouter.logger.Debugf("MessageRouter:ErrorChannel: Done sending error message.")
		}
	}
}

func (messageRouter *messageRouter) getDumpFor(appId string) []*logmessage.Message {
	if sink := messageRouter.activeSinks.DumpFor(appId); sink != nil {
		return sink.Dump()
	} else {
		messageRouter.logger.Debugf("MessageRouter:DumpReceiverChan: No dump exists for appId [%s].", appId)
		return []*logmessage.Message{}
	}
}

func (messageRouter *messageRouter) registerSink(s sinks.Sink) bool {
	messageRouter.Lock()
	defer messageRouter.Unlock()

	ok := messageRouter.activeSinks.Register(s)
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

func (messageRouter *messageRouter) unregisterSink(s sinks.Sink) {
	messageRouter.Lock()
	defer messageRouter.Unlock()

	messageRouter.activeSinks.Delete(s)
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

func (messageRouter *messageRouter) manageDumps(appId string) {
	if messageRouter.activeSinks.DumpFor(appId) != nil {
		return
	}

	s := sinks.NewDumpSink(appId, messageRouter.dumpBufferSize, messageRouter.logger)

	if messageRouter.registerSink(s) {
		go s.Run()
	}
}

func (messageRouter *messageRouter) manageDrains(appId string, drainUrls []string) {
	//delete all drains for app
	if len(drainUrls) == 0 {
		for _, sink := range messageRouter.activeSinks.DrainsFor(appId) {
			messageRouter.unregisterSink(sink)
		}
		return
	}

	//delete all drains that were not sent
	for _, sink := range messageRouter.activeSinks.DrainsFor(appId) {
		if contains(sink.Identifier(), drainUrls) {
			continue
		}
		messageRouter.unregisterSink(sink)
	}

	//add all drains that didn't exist
	for _, drainUrl := range drainUrls {
		if messageRouter.activeSinks.DrainFor(appId, drainUrl) == nil && !messageRouter.urlIsBlackListed(drainUrl) {
			parsedSyslogDrainUrl, err := checkURL(drainUrl, messageRouter.blackListIPs)
			if err != nil {
				messageRouter.blacklistedURLS = append(messageRouter.blacklistedURLS, drainUrl)
				errorMsg := fmt.Sprintf("MessageRouter: Invalid syslog drain URL: %s. Err: %v", drainUrl, err)
				messageRouter.sendLoggregatorErrorMessage(errorMsg, appId)
			} else {
				sysLogger := sinks.NewSyslogWriter(parsedSyslogDrainUrl.Scheme, parsedSyslogDrainUrl.Host, appId, messageRouter.skipCertVerify)
				s := sinks.NewSyslogSink(appId, drainUrl, messageRouter.logger, sysLogger, messageRouter.errorChannel)
				ok := messageRouter.registerSink(s)
				if ok {
					go s.Run()
				}
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

func checkURL(rawUrl string, blacklistedIPs []iprange.IPRange) (outputURL *url.URL, err error) {
	outputURL, err = url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}

	ipNotBlacklisted, err := iprange.IpOutsideOfRanges(*outputURL, blacklistedIPs)
	if err != nil {
		return nil, err
	}
	if !ipNotBlacklisted {
		return nil, errors.New("Syslog Drain URL is blacklisted")
	}
	return outputURL, nil
}
