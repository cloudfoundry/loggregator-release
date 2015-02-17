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
	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"sync"
	"time"
)

type SinkManager struct {
	DropsondeOrigin string

	Metrics        *metrics.SinkManagerMetrics
	recentLogCount uint32

	doneChannel            chan struct{}
	errorChannel           chan *events.Envelope
	urlBlacklistManager    *blacklist.URLBlacklistManager
	sinks                  *groupedsinks.GroupedSinks
	skipCertVerify         bool
	sinkTimeout, metricTTL time.Duration
	logger                 *gosteno.Logger
	stopOnce               sync.Once
	sync.RWMutex
}

func NewSinkManager(maxRetainedLogMessages uint32, skipCertVerify bool, blackListManager *blacklist.URLBlacklistManager, logger *gosteno.Logger, dropsondeOrigin string, sinkTimeout, metricTTL time.Duration) *SinkManager {
	return &SinkManager{
		doneChannel:         make(chan struct{}),
		errorChannel:        make(chan *events.Envelope, 100),
		urlBlacklistManager: blackListManager,
		sinks:               groupedsinks.NewGroupedSinks(logger),
		skipCertVerify:      skipCertVerify,
		recentLogCount:      maxRetainedLogMessages,
		Metrics:             metrics.NewSinkManagerMetrics(),
		logger:              logger,
		DropsondeOrigin:     dropsondeOrigin,
		sinkTimeout:         sinkTimeout,
		metricTTL:           metricTTL,
	}
}

func (sinkManager *SinkManager) Start(newAppServiceChan, deletedAppServiceChan <-chan appservice.AppService) {
	go sinkManager.listenForNewAppServices(newAppServiceChan)
	go sinkManager.listenForDeletedAppServices(deletedAppServiceChan)

	sinkManager.listenForErrorMessages()
}

func (sinkManager *SinkManager) Stop() {
	sinkManager.stopOnce.Do(func() {
		close(sinkManager.doneChannel)
		sinkManager.sinks.DeleteAll()
	})
}

func (sinkManager *SinkManager) SendTo(appId string, receivedMessage *events.Envelope) {
	sinkManager.ensureRecentLogsSinkFor(appId)
	sinkManager.ensureContainerMetricsSinkFor(appId)
	sinkManager.sinks.Broadcast(appId, receivedMessage)
}

func (sinkManager *SinkManager) RegisterSink(sink sinks.Sink) bool {
	inputChan := make(chan *events.Envelope, 128)
	ok := sinkManager.sinks.RegisterAppSink(inputChan, sink)
	if !ok {
		return false
	}

	sinkManager.Metrics.Inc(sink)

	sinkManager.logger.Debugf("SinkManager: Sink with identifier %v requested. Opened it.", sink.Identifier())

	go func() {
		sink.Run(inputChan)
		sinkManager.UnregisterSink(sink)
	}()

	return true
}

func (sinkManager *SinkManager) UnregisterSink(sink sinks.Sink) {

	ok := sinkManager.sinks.CloseAndDelete(sink)
	if !ok {
		return
	}
	sinkManager.Metrics.Dec(sink)

	if syslogSink, ok := sink.(*syslog.SyslogSink); ok {
		syslogSink.Disconnect()
	}

	sinkManager.logger.Debugf("SinkManager: Sink with identifier %s requested closing. Closed it.", sink.Identifier())
}

func (sinkManager *SinkManager) RegisterFirehoseSink(sink sinks.Sink) bool {
	inputChan := make(chan *events.Envelope, 1)
	ok := sinkManager.sinks.RegisterFirehoseSink(inputChan, sink)
	if !ok {
		return false
	}

	sinkManager.Metrics.IncFirehose()

	sinkManager.logger.Debugf("SinkManager: Firehose sink with identifier %v requested. Opened it.", sink.Identifier())

	go func() {
		sink.Run(inputChan)
	}()

	return true
}

func (sinkManager *SinkManager) UnregisterFirehoseSink(sink sinks.Sink) {
	ok := sinkManager.sinks.CloseAndDeleteFirehose(sink)
	if !ok {
		return
	}

	sinkManager.Metrics.DecFirehose()
	sinkManager.logger.Debugf("SinkManager: Firehose Sink with identifier %s requested closing. Closed it.", sink.Identifier())
}

func (sinkManager *SinkManager) RecentLogsFor(appId string) []*events.Envelope {
	if sink := sinkManager.sinks.DumpFor(appId); sink != nil {
		return sink.Dump()
	} else {
		sinkManager.logger.Debugf("SinkManager:DumpReceiverChan: No dump exists for appId [%s].", appId)
		return []*events.Envelope{}
	}
}

func (sinkManager *SinkManager) LatestContainerMetrics(appId string) []*events.Envelope {
	if sink := sinkManager.sinks.ContainerMetricsFor(appId); sink != nil {
		return sink.GetLatest()
	} else {
		sinkManager.logger.Debugf("SinkManager.LatestContainerMetrics: No container metrics exist for appId [%s].", appId)
		return []*events.Envelope{}
	}
}

func (sinkManager *SinkManager) SendSyslogErrorToLoggregator(errorMsg string, appId string, sinkUrl string) {
	sinkManager.Metrics.ReportSyslogError(appId, sinkUrl)

	sinkManager.logger.Warnf(errorMsg)

	logMessage := factories.NewLogMessage(events.LogMessage_ERR, errorMsg, appId, "LGR")

	envelope, err := emitter.Wrap(logMessage, sinkManager.DropsondeOrigin)

	if err != nil {
		sinkManager.logger.Warnf("Error marshalling message: %v", err)
		return
	}

	sinkManager.errorChannel <- envelope
}

func (sinkManager *SinkManager) Emit() instrumentation.Context {
	return sinkManager.Metrics.Emit()
}

func (sinkManager *SinkManager) listenForNewAppServices(newAppServiceChan <-chan appservice.AppService) {
	for appService := range newAppServiceChan {
		sinkManager.registerNewSyslogSink(appService.AppId, appService.Url)
	}
}

func (sinkManager *SinkManager) listenForDeletedAppServices(deletedAppServiceChan <-chan appservice.AppService) {
	for appService := range deletedAppServiceChan {
		syslogSink := sinkManager.sinks.DrainFor(appService.AppId, appService.Url)
		if syslogSink != nil {
			sinkManager.UnregisterSink(syslogSink)
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
			appId := envelope_extensions.GetAppId(errorMessage)
			sinkManager.logger.Debugf("SinkManager:ErrorChannel: Searching for sinks with appId [%s].", appId)
			sinkManager.sinks.BroadcastError(appId, errorMessage)
			sinkManager.logger.Debugf("SinkManager:ErrorChannel: Done sending error message.")
		}
	}
}

func (sinkManager *SinkManager) registerNewSyslogSink(appId string, syslogSinkUrl string) {
	parsedSyslogDrainUrl, err := sinkManager.urlBlacklistManager.CheckUrl(syslogSinkUrl)
	if err != nil {
		errorMsg := fmt.Sprintf("SinkManager: Invalid syslog drain URL (%s) for application %s. Err: %v", syslogSinkUrl, appId, err)
		sinkManager.SendSyslogErrorToLoggregator(errorMsg, appId, syslogSinkUrl)
	} else {
		syslogWriter := syslogwriter.NewSyslogWriter(parsedSyslogDrainUrl, appId, sinkManager.skipCertVerify)
		syslogSink := syslog.NewSyslogSink(appId, syslogSinkUrl, sinkManager.logger, syslogWriter, sinkManager.SendSyslogErrorToLoggregator, sinkManager.DropsondeOrigin)
		sinkManager.RegisterSink(syslogSink)
	}

}

func (sinkManager *SinkManager) ensureRecentLogsSinkFor(appId string) {
	if sinkManager.sinks.DumpFor(appId) != nil {
		return
	}

	sink := dump.NewDumpSink(appId, sinkManager.recentLogCount, sinkManager.logger, sinkManager.sinkTimeout)
	sinkManager.RegisterSink(sink)
}

func (sinkManager *SinkManager) ensureContainerMetricsSinkFor(appId string) {
	if sinkManager.sinks.ContainerMetricsFor(appId) != nil {
		return
	}

	sink := containermetric.NewContainerMetricSink(appId, sinkManager.metricTTL, sinkManager.sinkTimeout)
	sinkManager.RegisterSink(sink)
}
