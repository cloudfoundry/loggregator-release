package sinkmanager

import (
	"doppler/groupedsinks"
	"doppler/sinks"
	"doppler/sinks/containermetric"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/syslogwriter"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/metrics"
	"fmt"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/sonde-go/events"
)

type SinkManager struct {
	messageDrainBufferSize uint
	dropsondeOrigin        string

	metrics        *metrics.SinkManagerMetrics
	recentLogCount uint32

	doneChannel         chan struct{}
	errorChannel        chan *events.Envelope
	urlBlacklistManager *blacklist.URLBlacklistManager
	sinks               *groupedsinks.GroupedSinks
	skipCertVerify      bool
	sinkTimeout         time.Duration
	sinkIOTimeout       time.Duration
	metricTTL           time.Duration
	dialTimeout         time.Duration
	logger              *gosteno.Logger

	stopOnce sync.Once
}

func New(
	maxRetainedLogMessages uint32,
	skipCertVerify bool,
	blackListManager *blacklist.URLBlacklistManager,
	logger *gosteno.Logger,
	messageDrainBufferSize uint,
	dropsondeOrigin string,
	sinkTimeout,
	sinkIOTimeout,
	metricTTL,
	dialTimeout time.Duration,
) *SinkManager {
	return &SinkManager{
		doneChannel:            make(chan struct{}),
		errorChannel:           make(chan *events.Envelope, 100),
		urlBlacklistManager:    blackListManager,
		sinks:                  groupedsinks.NewGroupedSinks(logger),
		skipCertVerify:         skipCertVerify,
		recentLogCount:         maxRetainedLogMessages,
		metrics:                metrics.NewSinkManagerMetrics(),
		logger:                 logger,
		messageDrainBufferSize: messageDrainBufferSize,
		dropsondeOrigin:        dropsondeOrigin,
		sinkTimeout:            sinkTimeout,
		sinkIOTimeout:          sinkIOTimeout,
		metricTTL:              metricTTL,
		dialTimeout:            dialTimeout,
	}
}

func (sm *SinkManager) Start(newAppServiceChan, deletedAppServiceChan <-chan appservice.AppService) {
	go sm.listenForNewAppServices(newAppServiceChan)
	go sm.listenForDeletedAppServices(deletedAppServiceChan)

	sm.listenForErrorMessages()
}

func (sm *SinkManager) Stop() {
	sm.stopOnce.Do(func() {
		close(sm.doneChannel)
		sm.metrics.Stop()
		sm.sinks.DeleteAll()
	})
}

func (sm *SinkManager) SendTo(appID string, msg *events.Envelope) {
	sm.ensureRecentLogsSinkFor(appID)
	sm.ensureContainerMetricsSinkFor(appID)
	sm.sinks.Broadcast(appID, msg)
}

func (sm *SinkManager) RegisterSink(sink sinks.Sink) bool {
	inputChan := make(chan *events.Envelope, 128)
	ok := sm.sinks.RegisterAppSink(inputChan, sink)
	if !ok {
		return false
	}

	sm.metrics.Inc(sink)

	sm.logger.Debugf("SinkManager: Sink with identifier %v requested. Opened it.", sink.Identifier())

	go func() {
		sink.Run(inputChan)
		sm.UnregisterSink(sink)
	}()

	return true
}

func (sm *SinkManager) UnregisterSink(sink sinks.Sink) {

	ok := sm.sinks.CloseAndDelete(sink)
	if !ok {
		return
	}
	sm.metrics.Dec(sink)

	if syslogSink, ok := sink.(*syslog.SyslogSink); ok {
		syslogSink.Disconnect()
	}

	sm.logger.Debugf("SinkManager: Sink with identifier %s requested closing. Closed it.", sink.Identifier())
}

func (sm *SinkManager) IsFirehoseRegistered(sink sinks.Sink) bool {
	return sm.sinks.IsFirehoseRegistered(sink)
}

func (sm *SinkManager) RegisterFirehoseSink(sink sinks.Sink) bool {
	inputChan := make(chan *events.Envelope, 128)
	ok := sm.sinks.RegisterFirehoseSink(inputChan, sink)
	if !ok {
		return false
	}

	sm.metrics.IncFirehose()

	sm.logger.Debugf("SinkManager: Firehose sink with identifier %v requested. Opened it.", sink.Identifier())

	go func() {
		sink.Run(inputChan)
		sm.UnregisterFirehoseSink(sink)
	}()

	return true
}

func (sm *SinkManager) UnregisterFirehoseSink(sink sinks.Sink) {
	ok := sm.sinks.CloseAndDeleteFirehose(sink)
	if !ok {
		return
	}

	sm.metrics.DecFirehose()
	sm.logger.Debugf("SinkManager: Firehose Sink with identifier %s requested closing. Closed it.", sink.Identifier())
}

func (sm *SinkManager) RecentLogsFor(appId string) []*events.Envelope {
	if sink := sm.sinks.DumpFor(appId); sink != nil {
		return sink.Dump()
	} else {
		sm.logger.Debugf("SinkManager:DumpReceiverChan: No dump exists for appId [%s].", appId)
		return []*events.Envelope{}
	}
}

func (sm *SinkManager) LatestContainerMetrics(appId string) []*events.Envelope {
	if sink := sm.sinks.ContainerMetricsFor(appId); sink != nil {
		return sink.GetLatest()
	} else {
		sm.logger.Debugf("SinkManager.LatestContainerMetrics: No container metrics exist for appId [%s].", appId)
		return []*events.Envelope{}
	}
}

func (sm *SinkManager) SendSyslogErrorToLoggregator(errorMsg string, appId string) {
	sm.logger.Warn(errorMsg)

	logMessage := factories.NewLogMessage(events.LogMessage_ERR, errorMsg, appId, "LGR")
	envelope, err := emitter.Wrap(logMessage, sm.dropsondeOrigin)
	if err != nil {
		sm.logger.Warnf("Error marshalling message: %v", err)
		return
	}

	sm.errorChannel <- envelope
}

func (sm *SinkManager) listenForNewAppServices(newAppServiceChan <-chan appservice.AppService) {
	for {
		select {
		case <-sm.doneChannel:
			return
		case appService := <-newAppServiceChan:
			sm.registerNewSyslogSink(appService.AppId, appService.Url)
		}
	}
}

func (sm *SinkManager) listenForDeletedAppServices(deletedAppServiceChan <-chan appservice.AppService) {
	for {
		select {
		case <-sm.doneChannel:
			return
		case appService := <-deletedAppServiceChan:
			syslogSink := sm.sinks.DrainFor(appService.AppId, appService.Url)
			if syslogSink != nil {
				sm.UnregisterSink(syslogSink)
			}
		}
	}
}

func (sm *SinkManager) listenForErrorMessages() {
	for {
		select {
		case <-sm.doneChannel:
			return
		case errorMessage, ok := <-sm.errorChannel:
			if !ok {
				return
			}
			appId := envelope_extensions.GetAppId(errorMessage)
			sm.logger.Debugf("SinkManager:ErrorChannel: Searching for sinks with appId [%s].", appId)
			sm.sinks.BroadcastError(appId, errorMessage)
			sm.logger.Debugf("SinkManager:ErrorChannel: Done sending error message.")
		}
	}
}

func (sm *SinkManager) registerNewSyslogSink(appId string, syslogSinkURL string) {
	parsedSyslogDrainURL, err := sm.urlBlacklistManager.CheckUrl(syslogSinkURL)
	if err != nil {
		sm.SendSyslogErrorToLoggregator(invalidSyslogURLErrorMsg(appId, syslogSinkURL, err), appId)
		return
	}

	syslogWriter, err := syslogwriter.NewWriter(parsedSyslogDrainURL, appId, sm.skipCertVerify, sm.dialTimeout, sm.sinkIOTimeout)
	if err != nil {
		logURL := fmt.Sprintf("%s://%s%s", parsedSyslogDrainURL.Scheme, parsedSyslogDrainURL.Host, parsedSyslogDrainURL.Path)
		sm.SendSyslogErrorToLoggregator(invalidSyslogURLErrorMsg(appId, logURL, err), appId)
		return
	}

	syslogSink := syslog.NewSyslogSink(
		appId,
		parsedSyslogDrainURL,
		sm.logger,
		sm.messageDrainBufferSize,
		syslogWriter,
		sm.SendSyslogErrorToLoggregator,
		sm.dropsondeOrigin,
	)

	sm.RegisterSink(syslogSink)

}

func invalidSyslogURLErrorMsg(appId string, syslogSinkURL string, err error) string {
	return fmt.Sprintf("SinkManager: Invalid syslog drain URL (%s) for application %s. Err: %v", syslogSinkURL, appId, err)
}

func (sm *SinkManager) ensureRecentLogsSinkFor(appId string) {
	if sm.sinks.DumpFor(appId) != nil {
		return
	}

	sink := dump.NewDumpSink(
		appId,
		sm.recentLogCount,
		sm.logger,
		sm.sinkTimeout,
	)

	sm.RegisterSink(sink)
}

func (sm *SinkManager) ensureContainerMetricsSinkFor(appId string) {
	if sm.sinks.ContainerMetricsFor(appId) != nil {
		return
	}

	sink := containermetric.NewContainerMetricSink(
		appId,
		sm.metricTTL,
		sm.sinkTimeout,
	)

	sm.RegisterSink(sink)
}
