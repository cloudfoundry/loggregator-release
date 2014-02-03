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
	"time"
)

type SinkManager struct {
	activeDumpSinksCounter      int
	activeWebsocketSinksCounter int
	activeSyslogSinksCounter    int
	activeSinks                 *groupedsinks.GroupedSinks
	skipCertVerify              bool
	errorChannel                chan *logmessage.Message
	blackListIPs                []iprange.IPRange
	blacklistedURLS             []string
	logger                      *gosteno.Logger
	dumpBufferSize              int
	sinkCloseChan               chan sinks.Sink
	sinkOpenChan                chan sinks.Sink
	*sync.RWMutex
}

func NewSinkManager(maxRetainedLogMessages int, skipCertVerify bool, blackListIPs []iprange.IPRange, logger *gosteno.Logger) *SinkManager {
	sinkCloseChan := make(chan sinks.Sink, 20)
	sinkOpenChan := make(chan sinks.Sink, 20)
	errorChan := make(chan *logmessage.Message, 100)

	return &SinkManager{
		logger:         logger,
		RWMutex:        &sync.RWMutex{},
		skipCertVerify: skipCertVerify,
		activeSinks:    groupedsinks.NewGroupedSinks(),
		errorChannel:   errorChan,
		blackListIPs:   blackListIPs,
		dumpBufferSize: maxRetainedLogMessages,
		sinkCloseChan:  sinkCloseChan,
		sinkOpenChan:   sinkOpenChan,
	}
}

func (sinkManager *SinkManager) Start() {
	go func() {
		for {
			select {
			case sink := <-sinkManager.sinkOpenChan:
				sinkManager.registerSink(sink)
			case sink := <-sinkManager.sinkCloseChan:
				sinkManager.unregisterSink(sink)
			}
		}
	}()

	for {
		select {
		case receivedMessage := <-sinkManager.errorChannel:
			appId := receivedMessage.GetLogMessage().GetAppId()

			sinkManager.logger.Debugf("SinkManager:ErrorChannel: Searching for sinks with appId [%s].", appId)
		for _, sink := range sinkManager.activeSinks.For(appId) {
			if sink.ShouldReceiveErrors() {
				sinkManager.logger.Debugf("SinkManager:ErrorChannel: Sending Message to channel %v for sinks targeting [%s].", sink.Identifier(), appId)
				sink.Channel() <- receivedMessage
			}
		}
			sinkManager.logger.Debugf("SinkManager:ErrorChannel: Done sending error message.")
		}
	}
}

func (sinkManager *SinkManager) registerSink(s sinks.Sink) bool {
	sinkManager.Lock()
	defer sinkManager.Unlock()

	ok := sinkManager.activeSinks.Register(s)
	if !ok {
		return false
	}

	switch s.(type) {
	case *sinks.DumpSink:
		sinkManager.activeDumpSinksCounter++
	case *sinks.SyslogSink:
		sinkManager.activeSyslogSinksCounter++
	case *sinks.WebsocketSink:
		sinkManager.activeWebsocketSinksCounter++
	}
	sinkManager.logger.Infof("SinkManager: Sink with channel %v requested. Opened it.", s.Channel())
	return true
}

func (sinkManager *SinkManager) unregisterSink(s sinks.Sink) {
	sinkManager.Lock()
	defer sinkManager.Unlock()

	sinkManager.activeSinks.Delete(s)
	close(s.Channel())
	switch s.(type) {
	case *sinks.DumpSink:
		sinkManager.activeDumpSinksCounter--
	case *sinks.SyslogSink:
		syslogSink, _ := s.(*sinks.SyslogSink)
		syslogSink.Disconnect()
		sinkManager.activeSyslogSinksCounter--
	case *sinks.WebsocketSink:
		sinkManager.activeWebsocketSinksCounter--
	}
	sinkManager.logger.Infof("SinkManager: Sink with channel %v and identifier %s requested closing. Closed it.", s.Channel(), s.Identifier())
}

func (sinkManager *SinkManager) manageSyslogSinks(appId string, drainUrls []string) {
	//delete all drains for app
	if len(drainUrls) == 0 {
		for _, sink := range sinkManager.activeSinks.DrainsFor(appId) {
			sinkManager.unregisterSink(sink)
		}
		return
	}

	//delete all drains that were not sent
	for _, sink := range sinkManager.activeSinks.DrainsFor(appId) {
		if contains(sink.Identifier(), drainUrls) {
			continue
		}
		sinkManager.unregisterSink(sink)
	}

	//add all drains that didn't exist
	for _, drainUrl := range drainUrls {
		if sinkManager.activeSinks.DrainFor(appId, drainUrl) == nil && !sinkManager.urlIsBlackListed(drainUrl) {
			parsedSyslogDrainUrl, err := checkURL(drainUrl, sinkManager.blackListIPs)
			if err != nil {
				sinkManager.blacklistedURLS = append(sinkManager.blacklistedURLS, drainUrl)
				errorMsg := fmt.Sprintf("SinkManager: Invalid syslog drain URL: %s. Err: %v", drainUrl, err)
				sinkManager.sendLoggregatorErrorMessage(errorMsg, appId)
			} else {
				sysLogger := sinks.NewSyslogWriter(parsedSyslogDrainUrl.Scheme, parsedSyslogDrainUrl.Host, appId, sinkManager.skipCertVerify)
				s := sinks.NewSyslogSink(appId, drainUrl, sinkManager.logger, sysLogger, sinkManager.errorChannel)
				ok := sinkManager.registerSink(s)
				if ok {
					go s.Run()
				}
			}
		}
	}
}

func (sinkManager *SinkManager) getDumpFor(appId string) []*logmessage.Message {
	if sink := sinkManager.activeSinks.DumpFor(appId); sink != nil {
		return sink.Dump()
	} else {
		sinkManager.logger.Debugf("SinkManager:DumpReceiverChan: No dump exists for appId [%s].", appId)
		return []*logmessage.Message{}
	}
}

func (sinkManager *SinkManager) manageDumpSinks(appId string) {
	if sinkManager.activeSinks.DumpFor(appId) != nil {
		return
	}

	s := sinks.NewDumpSink(appId, sinkManager.dumpBufferSize, sinkManager.logger, sinkManager.sinkCloseChan, time.Hour)

	if sinkManager.registerSink(s) {
		go s.Run()
	}
}

func (sinkManager *SinkManager) sendLoggregatorErrorMessage(errorMsg, appId string) {
	sinkManager.logger.Warnf(errorMsg)
	logMessage, err := logmessage.GenerateMessage(logmessage.LogMessage_ERR, errorMsg, appId, "LGR")
	if err == nil {
		sinkManager.errorChannel <- logMessage
	} else {
		sinkManager.logger.Warnf("Error marshalling message: %v", err)
	}
}

func (sinkManager *SinkManager) urlIsBlackListed(testUrl string) bool {
	for _, url := range sinkManager.blacklistedURLS {
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

func (sinkManager *SinkManager) metrics() []instrumentation.Metric {
	sinkManager.RLock()
	defer sinkManager.RUnlock()

	return []instrumentation.Metric{
		instrumentation.Metric{Name: "numberOfDumpSinks", Value: sinkManager.activeDumpSinksCounter},
		instrumentation.Metric{Name: "numberOfSyslogSinks", Value: sinkManager.activeSyslogSinksCounter},
		instrumentation.Metric{Name: "numberOfWebsocketSinks", Value: sinkManager.activeWebsocketSinksCounter},
	}
}

func (sinkManager *SinkManager) Emit() instrumentation.Context {
	return instrumentation.Context{
		Name:    "messageRouter",
		Metrics: sinkManager.metrics(),
	}
}

func (sinkManager *SinkManager) sendToActiveSinks(appId string, receivedMessage *logmessage.Message) {
	for _, sink := range sinkManager.activeSinks.For(appId) {
		sinkManager.logger.Debugf("MessageRouter:ParsedMessageChan: Sending Message to channel %v for sinks targeting [%s].", sink.Identifier(), appId)
		sink.Channel() <- receivedMessage
	}
}
