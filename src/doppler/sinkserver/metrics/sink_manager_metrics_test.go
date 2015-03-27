package metrics_test

import (
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"doppler/sinkserver/metrics"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"

	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SinkManagerMetrics", func() {

	var sinkManagerMetrics *metrics.SinkManagerMetrics
	var sink sinks.Sink

	BeforeEach(func() {
		sinkManagerMetrics = metrics.NewSinkManagerMetrics()
	})

	It("emits metrics for dump sinks", func() {

		Expect(sinkManagerMetrics.Emit().Metrics[0].Name).To(Equal("numberOfDumpSinks"))
		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(uint64(0)))

		sink = &dump.DumpSink{}
		sinkManagerMetrics.Inc(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(uint64(1)))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(uint64(0)))

		sinkManagerMetrics.Dec(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(uint64(0)))
	})

	It("emits metrics for syslog sinks", func() {

		Expect(sinkManagerMetrics.Emit().Metrics[1].Name).To(Equal("numberOfSyslogSinks"))
		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(uint64(0)))

		sink := &syslog.SyslogSink{}
		sinkManagerMetrics.Inc(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(uint64(1)))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(uint64(0)))

		sinkManagerMetrics.Dec(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(uint64(0)))
	})

	It("emits metrics for websocket sinks", func() {

		Expect(sinkManagerMetrics.Emit().Metrics[2].Name).To(Equal("numberOfWebsocketSinks"))
		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(uint64(0)))

		sink := &websocket.WebsocketSink{}
		sinkManagerMetrics.Inc(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(uint64(1)))

		sinkManagerMetrics.Dec(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(uint64(0)))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(uint64(0)))
	})

	It("emits metrics for firehose sinks", func() {
		Expect(sinkManagerMetrics.Emit().Metrics[3].Name).To(Equal("numberOfFirehoseSinks"))
		Expect(sinkManagerMetrics.Emit().Metrics[3].Value).To(Equal(uint64(0)))

		sinkManagerMetrics.IncFirehose()
		Expect(sinkManagerMetrics.Emit().Metrics[3].Value).To(Equal(uint64(1)))

		sinkManagerMetrics.DecFirehose()
		Expect(sinkManagerMetrics.Emit().Metrics[3].Value).To(Equal(uint64(0)))

	})

	It("emits error counts for syslog sinks by app ID and drain URL", func() {
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() { // concurrent write
			defer wg.Done()
			sinkManagerMetrics.ReportSyslogError("app-id-1", "url-1")
		}()

		sinkManagerMetrics.ReportSyslogError("app-id-1", "url-2")
		sinkManagerMetrics.ReportSyslogError("app-id-1", "url-2")
		sinkManagerMetrics.ReportSyslogError("app-id-2", "url-3")

		Eventually(func() []instrumentation.Metric {
			return sinkManagerMetrics.Emit().Metrics[4:7]
		}).Should(ConsistOf(
			instrumentation.Metric{Name: "numberOfSyslogDrainErrors", Value: 1, Tags: map[string]interface{}{"appId": "app-id-1", "drainUrl": "url-1"}},
			instrumentation.Metric{Name: "numberOfSyslogDrainErrors", Value: 2, Tags: map[string]interface{}{"appId": "app-id-1", "drainUrl": "url-2"}},
			instrumentation.Metric{Name: "numberOfSyslogDrainErrors", Value: 1, Tags: map[string]interface{}{"appId": "app-id-2", "drainUrl": "url-3"}},
		))
	})

	It("emits dropped message counts by app id and drain url with total", func() {
		key1 := metrics.AppDrainMetricKey{AppId: "appId", DrainURL: "myApp"}
		sinkManagerMetrics.AppDrainMetrics[key1] = 25
		key2 := metrics.AppDrainMetricKey{AppId: "otherAppId", DrainURL: "myApp"}
		sinkManagerMetrics.AppDrainMetrics[key2] = 25

		Eventually(func() []instrumentation.Metric {
			return sinkManagerMetrics.Emit().Metrics[4:7]
		}).Should(ConsistOf(
			instrumentation.Metric{Name: "numberOfMessagesLost", Value: uint64(25), Tags: map[string]interface{}{"appId": "appId", "drainUrl": "myApp"}},
			instrumentation.Metric{Name: "numberOfMessagesLost", Value: uint64(25), Tags: map[string]interface{}{"appId": "otherAppId", "drainUrl": "myApp"}},
			instrumentation.Metric{Name: "totalDroppedMessages", Value: uint64(50)},
		))
	})

	Context("updates app drain metrics", func() {
		var updateMetric sinks.DrainMetric
		var key metrics.AppDrainMetricKey
		var waitForGoRoutines sync.WaitGroup
		BeforeEach(func() {
			waitForGoRoutines.Add(1)
			go func() {
				defer waitForGoRoutines.Done()
				sinkManagerMetrics.ListenForUpdatedAppDrainMetrics()
			}()
			key = metrics.AppDrainMetricKey{AppId: "appId", DrainURL: "localhost:9888"}
			sinkManagerMetrics.AppDrainMetrics[key] = 0

			updateMetric = sinks.DrainMetric{AppId: "appId", DrainURL: "localhost:9888", DroppedMsgCount: uint64(10)}
			sinkManagerMetrics.AppDrainMetricsReceiverChan <- updateMetric
		})

		AfterEach(func(done Done) {
			close(sinkManagerMetrics.AppDrainMetricsReceiverChan)
			waitForGoRoutines.Wait()
			close(done)
		})

		It("reads input and update appDrainMetrics array", func() {
			sinkManagerMetrics.AppDrainMetricsReceiverChan <- updateMetric

			Eventually(func() uint64 {
				sinkManagerMetrics.RLock()
				defer sinkManagerMetrics.RUnlock()
				return sinkManagerMetrics.AppDrainMetrics[key]
			}).Should(Equal(uint64(10)))
		})

		It("adds to AppDrainMetrics map", func() {
			sinkManagerMetrics.AddAppDrainEntry("appId", "localhost:9999")
			key := metrics.AppDrainMetricKey{AppId: "appId", DrainURL: "localhost:9999"}
			sinkManagerMetrics.RLock()
			Expect(sinkManagerMetrics.AppDrainMetrics).Should(HaveKeyWithValue(key, uint64(0)))
			sinkManagerMetrics.RUnlock()
		})

		It("removes from AppDrainMetrics map", func() {
			appId := "appIdRemove"
			drainURL := "localhost:9999"

			key := metrics.AppDrainMetricKey{AppId: appId, DrainURL: drainURL}
			sinkManagerMetrics.AddAppDrainEntry(appId, drainURL)

			sinkManagerMetrics.RemoveAppDrainEntry(appId, drainURL)
			sinkManagerMetrics.RLock()
			Expect(sinkManagerMetrics.AppDrainMetrics).ShouldNot(HaveKey(key))
			sinkManagerMetrics.RUnlock()
		})
	})

})
