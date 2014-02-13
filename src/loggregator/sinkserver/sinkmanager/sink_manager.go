package sinkmanager

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
	"loggregator/sinkserver/blacklist"
	"loggregator/sinkserver/metrics"
	"loggregator/sinks/syslog"
	"loggregator/sinks/dump"
	"github.com/cloudfoundry/gunk/timeprovider"
)

type SinkManager struct {
	errorChannel        chan *logmessage.Message
	urlBlacklistManager *blacklist.URLBlacklistManager
	sinks               *groupedsinks.GroupedSinks
	skipCertVerify      bool
	recentLogCount      uint32
	Metrics             *metrics.SinkManagerMetrics
	logger              *gosteno.Logger
}

func NewSinkManager(maxRetainedLogMessages uint32, skipCertVerify bool, blackListIPs []iprange.IPRange, logger *gosteno.Logger) *SinkManager {
	return &SinkManager{
		errorChannel:  make(chan *logmessage.Message, 100),
		urlBlacklistManager: blacklist.New(blackListIPs),
		sinks:          groupedsinks.NewGroupedSinks(),
		skipCertVerify: skipCertVerify,
		recentLogCount: maxRetainedLogMessages,
		Metrics:        metrics.NewSinkManagerMetrics(),
		logger:         logger,
	}
}

func (sinkManager *SinkManager) Start() {
	sinkManager.listenForErrorMessages()
}

func (sinkManager *SinkManager) Stop() {
}

func (sinkManager *SinkManager) SendTo(appId string, receivedMessage *logmessage.Message) {
	sinkManager.ensureRecentLogsSinkFor(appId)
	sinkManager.sinks.BroadCast(appId, receivedMessage)
}

func (sinkManager *SinkManager) listenForErrorMessages() {
	for errorMessage := range sinkManager.errorChannel {
		appId := errorMessage.GetLogMessage().GetAppId()
		sinkManager.logger.Debugf("SinkManager:ErrorChannel: Searching for sinks with appId [%s].", appId)
		sinkManager.sinks.BroadCastError(appId, errorMessage)
		sinkManager.logger.Debugf("SinkManager:ErrorChannel: Done sending error message.")
	}
}

func (sinkManager *SinkManager) RegisterSink(sink sinks.Sink, block ...bool) bool {
	inputChan := make(chan *logmessage.Message)
	ok := sinkManager.sinks.Register(inputChan, sink)
	if !ok {
		return false
	}

	sinkManager.Metrics.Inc(sink)

	sinkManager.logger.Infof("SinkManager: Sink with identifier %v requested. Opened it.", sink.Identifier())
	if len(block) > 0 {
		sink.Run(inputChan)
		sinkManager.unregisterSink(sink)
	} else {
		go func() {
			sink.Run(inputChan)
			sinkManager.unregisterSink(sink)
		}()
	}
	return true
}

func (sinkManager *SinkManager) unregisterSink(sink sinks.Sink) {
	sinkManager.sinks.Delete(sink)

	sinkManager.Metrics.Dec(sink)

	if syslogSink, ok := sink.(*syslog.SyslogSink); ok {
		syslogSink.Disconnect()
	}

	sinkManager.logger.Infof("SinkManager: Sink with channel %v and identifier %s requested closing. Closed it.", sink.Identifier())
}

func (sinkManager *SinkManager) ManageSyslogSinks(appId string, syslogSinkUrls []string) {
	if len(syslogSinkUrls) == 0 {
		sinkManager.unregisterAllSyslogSinks(appId)
		return
	}

	sinkManager.unregisterUnboundSyslogSinks(appId, syslogSinkUrls)

	sinkManager.registerNewSyslogSinks(appId, syslogSinkUrls)
}

func (sinkManager *SinkManager) unregisterAllSyslogSinks(appId string) {
	for _, sink := range sinkManager.sinks.DrainsFor(appId) {
		sinkManager.unregisterSink(sink)
	}
}

func (sinkManager *SinkManager) unregisterUnboundSyslogSinks(appId string, syslogSinkUrls []string) {
	for _, sink := range sinkManager.sinks.DrainsFor(appId) {
		if !contains(sink.Identifier(), syslogSinkUrls) {
			sinkManager.unregisterSink(sink)
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
				sinkManager.SendSyslogErrorToLoggregator(errorMsg, appId)
			} else {
				syslogWriter := syslogwriter.NewSyslogWriter(parsedSyslogDrainUrl, appId, sinkManager.skipCertVerify)
				syslogSink := syslog.NewSyslogSink(appId, syslogSinkUrl, sinkManager.logger, syslogWriter, sinkManager.errorChannel)
				sinkManager.RegisterSink(syslogSink)
			}
		}
	}
}

func (sinkManager *SinkManager) SendSyslogErrorToLoggregator(errorMsg, appId string) {
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

	s := dump.NewDumpSink(appId, sinkManager.recentLogCount, sinkManager.logger, time.Hour, timeprovider.NewTimeProvider())
	sinkManager.RegisterSink(s)
}

func (sinkManager *SinkManager) RecentLogsFor(appId string) []*logmessage.Message {
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
