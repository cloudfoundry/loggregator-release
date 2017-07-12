package sinkmanager

import (
	"fmt"
	"log"
	"sync"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/internal/groupedsinks"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/containermetric"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/dump"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/syslog"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks/syslogwriter"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/blacklist"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/metrics"
	"code.cloudfoundry.org/loggregator/metricemitter"

	"code.cloudfoundry.org/loggregator/doppler/internal/store"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
)

type MetricBatcher interface {
	BatchIncrementCounter(name string)
}

type HealthRegistrar interface {
	Inc(name string)
	Dec(name string)
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
}

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
	health              HealthRegistrar

	stopOnce sync.Once
}

func New(
	maxRetainedLogMessages uint32,
	skipCertVerify bool,
	blackListManager *blacklist.URLBlacklistManager,
	messageDrainBufferSize uint,
	dropsondeOrigin string,
	sinkTimeout,
	sinkIOTimeout,
	metricTTL,
	dialTimeout time.Duration,
	metricBatcher MetricBatcher,
	metricClient MetricClient,
	health HealthRegistrar,
) *SinkManager {
	return &SinkManager{
		doneChannel:            make(chan struct{}),
		errorChannel:           make(chan *events.Envelope, 100),
		urlBlacklistManager:    blackListManager,
		sinks:                  groupedsinks.NewGroupedSinks(metricBatcher, metricClient),
		skipCertVerify:         skipCertVerify,
		recentLogCount:         maxRetainedLogMessages,
		metrics:                metrics.NewSinkManagerMetrics(),
		messageDrainBufferSize: messageDrainBufferSize,
		dropsondeOrigin:        dropsondeOrigin,
		sinkTimeout:            sinkTimeout,
		sinkIOTimeout:          sinkIOTimeout,
		metricTTL:              metricTTL,
		dialTimeout:            dialTimeout,
		health:                 health,
	}
}

func (sm *SinkManager) Start(newAppServiceChan, deletedAppServiceChan <-chan store.AppService) {
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

	// metric-documentation-v1: see sink_manager_metrics.go for details
	sm.metrics.Inc(sink)

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
}

func (sm *SinkManager) RecentLogsFor(appId string) []*events.Envelope {
	if sink := sm.sinks.DumpFor(appId); sink != nil {
		return sink.Dump()
	}

	return nil
}

func (sm *SinkManager) LatestContainerMetrics(appId string) []*events.Envelope {
	if sink := sm.sinks.ContainerMetricsFor(appId); sink != nil {
		return sink.GetLatest()
	} else {
		return []*events.Envelope{}
	}
}

func (sm *SinkManager) SendSyslogErrorToLoggregator(errorMsg string, appId string) {
	log.Printf("SendSyslogError: %s", errorMsg)

	logMessage := factories.NewLogMessage(events.LogMessage_ERR, errorMsg, appId, "LGR")
	envelope, err := emitter.Wrap(logMessage, sm.dropsondeOrigin)
	if err != nil {
		log.Printf("Error marshalling message: %v", err)
		return
	}

	sm.errorChannel <- envelope
}

func (sm *SinkManager) listenForNewAppServices(newAppServiceChan <-chan store.AppService) {
	for {
		select {
		case <-sm.doneChannel:
			return
		case appService := <-newAppServiceChan:
			sm.registerNewSyslogSink(appService.AppId(), appService.Url(), appService.Hostname())
		}
	}
}

func (sm *SinkManager) listenForDeletedAppServices(deletedAppServiceChan <-chan store.AppService) {
	for {
		select {
		case <-sm.doneChannel:
			return
		case appService := <-deletedAppServiceChan:
			syslogSink := sm.sinks.DrainFor(appService.AppId(), appService.Url())
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
			sm.sinks.BroadcastError(appId, errorMessage)
		}
	}
}

func (sm *SinkManager) registerNewSyslogSink(appId, syslogSinkURL, hostname string) {
	parsedSyslogDrainURL, err := sm.urlBlacklistManager.CheckUrl(syslogSinkURL)
	if err != nil {
		sm.SendSyslogErrorToLoggregator(invalidSyslogURLErrorMsg(appId, syslogSinkURL, err), appId)
		return
	}

	syslogWriter, err := syslogwriter.NewWriter(
		parsedSyslogDrainURL,
		appId,
		hostname,
		sm.skipCertVerify,
		sm.dialTimeout,
		sm.sinkIOTimeout,
	)
	if err != nil {
		logURL := fmt.Sprintf("%s://%s%s", parsedSyslogDrainURL.Scheme, parsedSyslogDrainURL.Host, parsedSyslogDrainURL.Path)
		sm.SendSyslogErrorToLoggregator(invalidSyslogURLErrorMsg(appId, logURL, err), appId)
		return
	}

	syslogSink := syslog.NewSyslogSink(
		appId,
		parsedSyslogDrainURL,
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
		sm.sinkTimeout,
		sm.health,
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
		sm.health,
	)

	sm.RegisterSink(sink)
}
