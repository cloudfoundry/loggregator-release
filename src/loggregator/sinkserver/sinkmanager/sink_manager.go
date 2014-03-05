package sinkmanager

import (
	"fmt"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/domain"
	"loggregator/groupedsinks"
	"loggregator/sinks"
	"loggregator/sinks/dump"
	"loggregator/sinks/syslog"
	"loggregator/sinks/syslogwriter"
	"loggregator/sinkserver/blacklist"
	"loggregator/sinkserver/metrics"
	"sync"
	"time"
)

type SinkManager struct {
	sync.RWMutex
	doneChannel         chan struct{}
	errorChannel        chan *logmessage.Message
	urlBlacklistManager *blacklist.URLBlacklistManager
	sinks               *groupedsinks.GroupedSinks
	skipCertVerify      bool
	recentLogCount      uint32
	Metrics             *metrics.SinkManagerMetrics
	logger              *gosteno.Logger
	appStoreUpdateChan  chan<- domain.AppServices
	stopped             bool
}

func NewSinkManager(maxRetainedLogMessages uint32, skipCertVerify bool, blackListManager *blacklist.URLBlacklistManager, logger *gosteno.Logger) (*SinkManager, <-chan domain.AppServices) {
	appStoreUpdateChan := make(chan domain.AppServices, 10)
	return &SinkManager{
		doneChannel:         make(chan struct{}),
		errorChannel:        make(chan *logmessage.Message, 100),
		urlBlacklistManager: blackListManager,
		sinks:               groupedsinks.NewGroupedSinks(),
		skipCertVerify:      skipCertVerify,
		recentLogCount:      maxRetainedLogMessages,
		Metrics:             metrics.NewSinkManagerMetrics(),
		logger:              logger,
		appStoreUpdateChan:  appStoreUpdateChan,
	}, appStoreUpdateChan
}

func (sinkManager *SinkManager) Start(newAppServiceChan, deletedAppServiceChan <-chan domain.AppService) {
	sinkManager.setStopped(false)
	go sinkManager.listenForNewAppServices(newAppServiceChan)
	go sinkManager.listenForDeletedAppServices(deletedAppServiceChan)

	sinkManager.listenForErrorMessages()
}

func (sinkManager *SinkManager) Stop() {
	sinkManager.setStopped(true)
	select {
	case <-sinkManager.doneChannel:
	default:
		close(sinkManager.doneChannel)
		sinkManager.sinks.DeleteAll()
		//		close(sinkManager.appStoreUpdateChan)
	}
}

func (sinkManager *SinkManager) SendTo(appId string, receivedMessage *logmessage.Message) {
	sinkManager.ensureRecentLogsSinkFor(appId)
	sinkManager.sinks.BroadCast(appId, receivedMessage)
}

func (sinkManager *SinkManager) listenForNewAppServices(newAppServiceChan <-chan domain.AppService) {
	for appService := range newAppServiceChan {
		sinkManager.registerNewSyslogSink(appService.AppId, appService.Url)
	}
}

func (sinkManager *SinkManager) listenForDeletedAppServices(deletedAppServiceChan <-chan domain.AppService) {
	for appService := range deletedAppServiceChan {
		syslogSink := sinkManager.sinks.DrainFor(appService.AppId, appService.Url)
		if syslogSink != nil {
			sinkManager.unregisterSink(syslogSink)
		}
	}
}

func (sinkManager *SinkManager) listenForErrorMessages() {
	for {
		select {
		case <-sinkManager.doneChannel:
			return
		case errorMessage, ok := <-sinkManager.errorChannel:
			if !ok {
				return
			}
			appId := errorMessage.GetLogMessage().GetAppId()
			sinkManager.logger.Debugf("SinkManager:ErrorChannel: Searching for sinks with appId [%s].", appId)
			sinkManager.sinks.BroadCastError(appId, errorMessage)
			sinkManager.logger.Debugf("SinkManager:ErrorChannel: Done sending error message.")
		}
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

	ok := sinkManager.sinks.Delete(sink)
	if !ok {
		return
	}
	sinkManager.Metrics.Dec(sink)

	if syslogSink, ok := sink.(*syslog.SyslogSink); ok {
		syslogSink.Disconnect()
	} else if _, ok := sink.(*dump.DumpSink); ok {
		sinkManager.appStoreUpdateChan <- domain.AppServices{AppId: sink.AppId()}
	}

	sinkManager.logger.Infof("SinkManager: Sink with channel %v and identifier %s requested closing. Closed it.", sink.Identifier())
}

func (sinkManager *SinkManager) setStopped(v bool) {
	sinkManager.Lock()
	defer sinkManager.Unlock()
	sinkManager.stopped = v
}

func (sinkManager *SinkManager) isStopped() bool {
	sinkManager.RLock()
	defer sinkManager.RUnlock()
	return sinkManager.stopped
}

func (sinkManager *SinkManager) ManageSyslogSinks(appId string, syslogSinkUrls []string) {
	if sinkManager.isStopped() {
		return
	}
	sinkManager.appStoreUpdateChan <- domain.AppServices{AppId: appId, Urls: syslogSinkUrls}
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

func (sinkManager *SinkManager) registerNewSyslogSink(appId string, syslogSinkUrl string) {
	parsedSyslogDrainUrl, err := sinkManager.urlBlacklistManager.CheckUrl(syslogSinkUrl)
	if err != nil {
		errorMsg := fmt.Sprintf("SinkManager: Invalid syslog drain URL: %s. Err: %v", syslogSinkUrl, err)
		sinkManager.SendSyslogErrorToLoggregator(errorMsg, appId)
	} else {
		syslogWriter := syslogwriter.NewSyslogWriter(parsedSyslogDrainUrl, appId, sinkManager.skipCertVerify)
		syslogSink := syslog.NewSyslogSink(appId, syslogSinkUrl, sinkManager.logger, syslogWriter, sinkManager.errorChannel)
		sinkManager.RegisterSink(syslogSink)
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
