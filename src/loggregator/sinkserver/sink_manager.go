package sinkserver

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/groupedsinks"
	"loggregator/iprange"
	"loggregator/sinks"
	"loggregator/sinks/syslogwriter"
	"time"
)

type SinkManager struct {
	sinkOpenChan        chan sinks.Sink
	sinkCloseChan       chan sinks.Sink
	errorChannel        chan *logmessage.Message
	urlBlacklistManager *URLBlacklistManager
	sinks               *groupedsinks.GroupedSinks
	skipCertVerify      bool
	recentLogCount      int
	Metrics             *SinkManagerMetrics
	logger              *gosteno.Logger
}

func NewSinkManager(maxRetainedLogMessages int, skipCertVerify bool, blackListIPs []iprange.IPRange, logger *gosteno.Logger) *SinkManager {
	return &SinkManager{
		sinkOpenChan:  make(chan sinks.Sink, 20),
		sinkCloseChan: make(chan sinks.Sink, 20),
		errorChannel:  make(chan *logmessage.Message, 100),
		urlBlacklistManager: &URLBlacklistManager{
			blacklistIPs: blackListIPs,
		},
		sinks:          groupedsinks.NewGroupedSinks(),
		skipCertVerify: skipCertVerify,
		recentLogCount: maxRetainedLogMessages,
		Metrics:        NewSinkManagerMetrics(),
		logger:         logger,
	}
}

func (sinkManager *SinkManager) Start() {
	go sinkManager.listenForSinkChanges()

	sinkManager.listenForErrorMessages()
}

func (sinkManager *SinkManager) Stop() {
}

func (sinkManager *SinkManager) SendTo(appId string, receivedMessage *logmessage.Message) {
	for _, sink := range sinkManager.sinks.For(appId) {
		sinkManager.logger.Debugf("MessageRouter:ParsedMessageChan: Sending Message to channel %v for sinks targeting [%s].", sink.Identifier(), appId)
		sink.Channel() <- receivedMessage
	}
}

func (sinkManager *SinkManager) listenForSinkChanges() {
	for {
		select {
		case sink := <-sinkManager.sinkOpenChan:
			sinkManager.RegisterSink(sink)
		case sink := <-sinkManager.sinkCloseChan:
			sinkManager.UnregisterSink(sink)
		}
	}
}

func (sinkManager *SinkManager) listenForErrorMessages() {
	for errorMessage := range sinkManager.errorChannel {
		appId := errorMessage.GetLogMessage().GetAppId()
		sinkManager.logger.Debugf("SinkManager:ErrorChannel: Searching for sinks with appId [%s].", appId)

		for _, sink := range sinkManager.sinks.For(appId) {
			if sink.ShouldReceiveErrors() {
				sinkManager.logger.Debugf("SinkManager:ErrorChannel: Sending Message to channel %v for sinks targeting [%s].", sink.Identifier(), appId)
				sink.Channel() <- errorMessage
			}
		}
		sinkManager.logger.Debugf("SinkManager:ErrorChannel: Done sending error message.")
	}
}

func (sinkManager *SinkManager) RegisterSink(sink sinks.Sink) bool {
	ok := sinkManager.sinks.Register(sink)
	if !ok {
		return false
	}

	sinkManager.Metrics.Inc(sink)

	sinkManager.logger.Infof("SinkManager: Sink with channel %v requested. Opened it.", sink.Channel())
	return true
}

func (sinkManager *SinkManager) UnregisterSink(sink sinks.Sink) {
	sinkManager.sinks.Delete(sink)
	close(sink.Channel())

	sinkManager.Metrics.Dec(sink)

	if syslogSink, ok := sink.(*sinks.SyslogSink); ok {
		syslogSink.Disconnect()
	}

	sinkManager.logger.Infof("SinkManager: Sink with channel %v and identifier %s requested closing. Closed it.", sink.Channel(), sink.Identifier())
}

func (sinkManager *SinkManager) manageSyslogSinks(appId string, syslogSinkUrls []string) {
	if len(syslogSinkUrls) == 0 {
		sinkManager.unregisterAllSyslogSinks(appId)
		return
	}

	sinkManager.unregisterUnboundSyslogSinks(appId, syslogSinkUrls)

	sinkManager.registerNewSyslogSinks(appId, syslogSinkUrls)
}

func (sinkManager *SinkManager) unregisterAllSyslogSinks(appId string) {
	for _, sink := range sinkManager.sinks.DrainsFor(appId) {
		sinkManager.UnregisterSink(sink)
	}
}

func (sinkManager *SinkManager) unregisterUnboundSyslogSinks(appId string, syslogSinkUrls []string) {
	for _, sink := range sinkManager.sinks.DrainsFor(appId) {
		if !contains(sink.Identifier(), syslogSinkUrls) {
			sinkManager.UnregisterSink(sink)
		}
	}
}

func (sinkManager *SinkManager) registerNewSyslogSinks(appId string, syslogSinkUrls []string) {
	for _, syslogSinkUrl := range syslogSinkUrls {
		if sinkManager.sinks.DrainFor(appId, syslogSinkUrl) == nil && !sinkManager.urlBlacklistManager.IsBlacklisted(syslogSinkUrl) {
			parsedSyslogDrainUrl, err := sinkManager.urlBlacklistManager.CheckUrl(syslogSinkUrl)
			if err != nil {
				sinkManager.urlBlacklistManager.BlacklistUrl(syslogSinkUrl)
				errorMsg := fmt.Sprintf("SinkManager: Invalid syslog drain URL: %s. Err: %v", syslogSinkUrl, err)
				sinkManager.sendSyslogErrorToLoggregator(errorMsg, appId)
			} else {
				syslogWriter := syslogwriter.NewSyslogWriter(parsedSyslogDrainUrl, appId, sinkManager.skipCertVerify)
				syslogSink := sinks.NewSyslogSink(appId, syslogSinkUrl, sinkManager.logger, syslogWriter, sinkManager.errorChannel)
				if sinkManager.RegisterSink(syslogSink) {
					go syslogSink.Run()
				}
			}
		}
	}
}

func (sinkManager *SinkManager) sendSyslogErrorToLoggregator(errorMsg, appId string) {
	sinkManager.logger.Warnf(errorMsg)
	logMessage, err := logmessage.GenerateMessage(logmessage.LogMessage_ERR, errorMsg, appId, "LGR")
	if err == nil {
		sinkManager.errorChannel <- logMessage
	} else {
		sinkManager.logger.Warnf("Error marshalling message: %v", err)
	}
}

func (sinkManager *SinkManager) ensureRecentLogsSinkFor(appId string) {
	if sinkManager.sinks.DumpFor(appId) != nil {
		return
	}

	s := sinks.NewDumpSink(appId, sinkManager.recentLogCount, sinkManager.logger, sinkManager.sinkCloseChan, time.Hour)

	if sinkManager.RegisterSink(s) {
		go s.Run()
	}
}

func (sinkManager *SinkManager) recentLogsFor(appId string) []*logmessage.Message {
	if sink := sinkManager.sinks.DumpFor(appId); sink != nil {
		return sink.Dump()
	} else {
		sinkManager.logger.Debugf("SinkManager:DumpReceiverChan: No dump exists for appId [%s].", appId)
		return []*logmessage.Message{}
	}
}

func (sinkManager *SinkManager) Emit() instrumentation.Context {
	return sinkManager.Metrics.Emit()
}

func contains(valueToFind string, values []string) bool {
	for _, value := range values {
		if valueToFind == value {
			return true
		}
	}
	return false
}
