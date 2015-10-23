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

func New(maxRetainedLogMessages uint32, skipCertVerify bool, blackListManager *blacklist.URLBlacklistManager, logger *gosteno.Logger, messageDrainBufferSize uint, dropsondeOrigin string, sinkTimeout, sinkIOTimeout, metricTTL, dialTimeout time.Duration) *SinkManager {
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

	sinkManager.metrics.Inc(sink)

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
	sinkManager.metrics.Dec(sink)

	if syslogSink, ok := sink.(*syslog.SyslogSink); ok {
		syslogSink.Disconnect()
	}

	sinkManager.logger.Debugf("SinkManager: Sink with identifier %s requested closing. Closed it.", sink.Identifier())
}

func (sinkManager *SinkManager) IsFirehoseRegistered(sink sinks.Sink) bool {
	return sinkManager.sinks.IsFirehoseRegistered(sink)
}

func (sinkManager *SinkManager) RegisterFirehoseSink(sink sinks.Sink) bool {
	inputChan := make(chan *events.Envelope, 128)
	ok := sinkManager.sinks.RegisterFirehoseSink(inputChan, sink)
	if !ok {
		return false
	}

	sinkManager.metrics.IncFirehose()

	sinkManager.logger.Debugf("SinkManager: Firehose sink with identifier %v requested. Opened it.", sink.Identifier())

	go func() {
		sink.Run(inputChan)
		sinkManager.UnregisterFirehoseSink(sink)
	}()

	return true
}

func (sinkManager *SinkManager) UnregisterFirehoseSink(sink sinks.Sink) {
	ok := sinkManager.sinks.CloseAndDeleteFirehose(sink)
	if !ok {
		return
	}

	sinkManager.metrics.DecFirehose()
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
	sinkManager.logger.Warnf("%s", errorMsg)

	logMessage := factories.NewLogMessage(events.LogMessage_ERR, errorMsg, appId, "LGR")

	envelope, err := emitter.Wrap(logMessage, sinkManager.dropsondeOrigin)

	if err != nil {
		sinkManager.logger.Warnf("Error marshalling message: %v", err)
		return
	}

	sinkManager.errorChannel <- envelope
}

func (sinkManager *SinkManager) listenForNewAppServices(newAppServiceChan <-chan appservice.AppService) {
	for {
		select {
		case <-sinkManager.doneChannel:
			return
		case appService := <-newAppServiceChan:
			sinkManager.registerNewSyslogSink(appService.AppId, appService.Url)
		}
	}
}

func (sinkManager *SinkManager) listenForDeletedAppServices(deletedAppServiceChan <-chan appservice.AppService) {
	for {
		select {
		case <-sinkManager.doneChannel:
			return
		case appService := <-deletedAppServiceChan:
			syslogSink := sinkManager.sinks.DrainFor(appService.AppId, appService.Url)
			if syslogSink != nil {
				sinkManager.UnregisterSink(syslogSink)
			}
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
		sinkManager.SendSyslogErrorToLoggregator(invalidSyslogUrlErrorMsg(appId, syslogSinkUrl, err), appId, syslogSinkUrl)
		return
	}

	syslogWriter, err := syslogwriter.NewWriter(parsedSyslogDrainUrl, appId, sinkManager.skipCertVerify, sinkManager.dialTimeout, sinkManager.sinkIOTimeout)
	if err != nil {
		sinkManager.SendSyslogErrorToLoggregator(invalidSyslogUrlErrorMsg(appId, syslogSinkUrl, err), appId, syslogSinkUrl)
		return
	}

	syslogSink := syslog.NewSyslogSink(
		appId,
		syslogSinkUrl,
		sinkManager.logger,
		sinkManager.messageDrainBufferSize,
		syslogWriter,
		sinkManager.SendSyslogErrorToLoggregator,
		sinkManager.dropsondeOrigin,
	)

	sinkManager.RegisterSink(syslogSink)

}

func invalidSyslogUrlErrorMsg(appId string, syslogSinkUrl string, err error) string {
	return fmt.Sprintf("SinkManager: Invalid syslog drain URL (%s) for application %s. Err: %v", syslogSinkUrl, appId, err)
}

func (sinkManager *SinkManager) ensureRecentLogsSinkFor(appId string) {
	if sinkManager.sinks.DumpFor(appId) != nil {
		return
	}

	sink := dump.NewDumpSink(
		appId,
		sinkManager.recentLogCount,
		sinkManager.logger,
		sinkManager.sinkTimeout,
	)

	sinkManager.RegisterSink(sink)
}

func (sinkManager *SinkManager) ensureContainerMetricsSinkFor(appId string) {
	if sinkManager.sinks.ContainerMetricsFor(appId) != nil {
		return
	}

	sink := containermetric.NewContainerMetricSink(
		appId,
		sinkManager.metricTTL,
		sinkManager.sinkTimeout,
	)

	sinkManager.RegisterSink(sink)
}
