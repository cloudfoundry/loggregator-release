package sinks

import (
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/envelope_extensions"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
)

// SinkManager manages the lifecycle of a syslog sink. It also provides an
// in memory store of recent logs and container metrics.
type SinkManager struct {
	messageDrainBufferSize uint
	dropsondeOrigin        string
	metrics                *SinkManagerMetrics
	recentLogCount         uint32
	doneChannel            chan struct{}
	errorChannel           chan *events.Envelope
	urlBlacklistManager    *URLBlacklistManager
	sinks                  *GroupedSinks
	skipCertVerify         bool
	sinkTimeout            time.Duration
	sinkIOTimeout          time.Duration
	metricTTL              time.Duration
	dialTimeout            time.Duration
	health                 HealthRegistrar
	stopOnce               sync.Once
}

// NewSinkManager creates a SinkManager.
func NewSinkManager(
	maxRetainedLogMessages uint32,
	skipCertVerify bool,
	blackListManager *URLBlacklistManager,
	messageDrainBufferSize uint,
	dropsondeOrigin string,
	sinkTimeout time.Duration,
	sinkIOTimeout time.Duration,
	metricTTL time.Duration,
	dialTimeout time.Duration,
	metricBatcher MetricBatcher,
	metricClient MetricClient,
	health HealthRegistrar,
) *SinkManager {
	return &SinkManager{
		doneChannel:            make(chan struct{}),
		errorChannel:           make(chan *events.Envelope, 100),
		urlBlacklistManager:    blackListManager,
		sinks:                  NewGroupedSinks(metricBatcher, metricClient),
		skipCertVerify:         skipCertVerify,
		recentLogCount:         maxRetainedLogMessages,
		metrics:                NewSinkManagerMetrics(),
		messageDrainBufferSize: messageDrainBufferSize,
		dropsondeOrigin:        dropsondeOrigin,
		sinkTimeout:            sinkTimeout,
		sinkIOTimeout:          sinkIOTimeout,
		metricTTL:              metricTTL,
		dialTimeout:            dialTimeout,
		health:                 health,
	}
}

// Start will being monitoring both channels for created or deleted syslog
// drains bound to application logs.
func (sm *SinkManager) Start(newAppServiceChan, deletedAppServiceChan <-chan AppService) {
	go sm.listenForNewAppServices(newAppServiceChan)
	go sm.listenForDeletedAppServices(deletedAppServiceChan)

	sm.listenForErrorMessages()
}

// Stop terminates the sink manager.
func (sm *SinkManager) Stop() {
	sm.stopOnce.Do(func() {
		close(sm.doneChannel)
		sm.metrics.Stop()
		sm.sinks.DeleteAll()
	})
}

// SendTo sends an envelope to the registered sinks for a specified
// application ID.
func (sm *SinkManager) SendTo(appID string, msg *events.Envelope) {
	sm.ensureRecentLogsSinkFor(appID)
	sm.ensureContainerMetricsSinkFor(appID)
	sm.sinks.Broadcast(appID, msg)
}

// RegisterSink sink adds a new sink for the sink manager to manage.
//
// FIXME This method should be private. Nothing calls it except for private
// functions in this file.
func (sm *SinkManager) RegisterSink(sink Sink) bool {
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

// UnregisterSink removes a particular sink from the sink manager.
//
// FIXME This method should be private. Nothing calls it except for private
// functions in this file.
func (sm *SinkManager) UnregisterSink(sink Sink) {

	ok := sm.sinks.CloseAndDelete(sink)
	if !ok {
		return
	}
	sm.metrics.Dec(sink)

	if syslogSink, ok := sink.(*SyslogSink); ok {
		syslogSink.Disconnect()
	}
}

// RecentLogsFor provides a fixed number of logs for an application ID.
func (sm *SinkManager) RecentLogsFor(appID string) []*events.Envelope {
	if sink := sm.sinks.DumpFor(appID); sink != nil {
		return sink.Dump()
	}

	return nil
}

// LatestContainerMetrics returns the most recent container metrics for an
// application ID.
func (sm *SinkManager) LatestContainerMetrics(appID string) []*events.Envelope {
	if sink := sm.sinks.ContainerMetricsFor(appID); sink != nil {
		return sink.GetLatest()
	}
	return []*events.Envelope{}
}

// SendSyslogErrorToLoggregator reports a given error for an application ID.
//
// FIXME This method should be private. Nothing calls it except for private
// functions in this file.
func (sm *SinkManager) SendSyslogErrorToLoggregator(errorMsg string, appID string) {
	log.Printf("SendSyslogError: %s", errorMsg)

	logMessage := factories.NewLogMessage(events.LogMessage_ERR, errorMsg, appID, "LGR")
	envelope, err := emitter.Wrap(logMessage, sm.dropsondeOrigin)
	if err != nil {
		log.Printf("Error marshalling message: %v", err)
		return
	}

	sm.errorChannel <- envelope
}

func (sm *SinkManager) listenForNewAppServices(createStream <-chan AppService) {
	for {
		select {
		case <-sm.doneChannel:
			return
		case appService := <-createStream:
			u, err := sm.urlBlacklistManager.CheckUrl(appService.Url())
			if err != nil {
				errMsg := invalidSyslogURLErrorMsg(
					appService.AppId(),
					appService.Url(),
					err,
				)
				sm.SendSyslogErrorToLoggregator(errMsg, appService.AppId())
				continue
			}

			sm.registerNewSyslogSink(
				appService.AppId(),
				u,
				appService.Hostname(),
			)
		}
	}
}

func (sm *SinkManager) listenForDeletedAppServices(deleteStream <-chan AppService) {
	for {
		select {
		case <-sm.doneChannel:
			return
		case appService := <-deleteStream:
			u, err := url.Parse(appService.Url())
			if err != nil {
				// If the app service URL is invalid, it will not have been
				// registered above. There is no need for any further work and
				// the parse error may be ignored.
				continue
			}
			key := IdentifierFromURL(u)
			syslogSink := sm.sinks.DrainFor(appService.AppId(), key)

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
			appID := envelope_extensions.GetAppId(errorMessage)
			sm.sinks.BroadcastError(appID, errorMessage)
		}
	}
}

func (sm *SinkManager) registerNewSyslogSink(appID string, syslogSinkURL *url.URL, hostname string) {
	syslogWriter, err := NewWriter(
		syslogSinkURL,
		appID,
		hostname,
		sm.skipCertVerify,
		sm.dialTimeout,
		sm.sinkIOTimeout,
	)
	if err != nil {
		logURL := fmt.Sprintf("%s://%s%s", syslogSinkURL.Scheme, syslogSinkURL.Host, syslogSinkURL.Path)
		sm.SendSyslogErrorToLoggregator(invalidSyslogURLErrorMsg(appID, logURL, err), appID)
		return
	}

	syslogSink := NewSyslogSink(
		appID,
		syslogSinkURL,
		sm.messageDrainBufferSize,
		syslogWriter,
		sm.SendSyslogErrorToLoggregator,
		sm.dropsondeOrigin,
	)

	sm.RegisterSink(syslogSink)
}

func invalidSyslogURLErrorMsg(appID string, syslogSinkURL string, err error) string {
	return fmt.Sprintf("SinkManager: Invalid syslog drain URL (%s) for application %s. Err: %v", syslogSinkURL, appID, err)
}

func (sm *SinkManager) ensureRecentLogsSinkFor(appID string) {
	if sm.sinks.DumpFor(appID) != nil {
		return
	}

	sink := NewDumpSink(
		appID,
		sm.recentLogCount,
		sm.sinkTimeout,
		sm.health,
	)

	sm.RegisterSink(sink)
}

func (sm *SinkManager) ensureContainerMetricsSinkFor(appID string) {
	if sm.sinks.ContainerMetricsFor(appID) != nil {
		return
	}

	sink := NewContainerMetricSink(
		appID,
		sm.metricTTL,
		sm.sinkTimeout,
		sm.health,
	)

	sm.RegisterSink(sink)
}
