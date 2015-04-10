package metrics_test

import (
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"doppler/sinkserver/metrics"

	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SinkManagerMetrics", func() {

	var sinkManagerMetrics *metrics.SinkManagerMetrics
	var sink sinks.Sink
	var dropUpdateChan chan int64

	BeforeEach(func() {
		dropUpdateChan = make(chan int64)
		sinkManagerMetrics = metrics.NewSinkManagerMetrics(dropUpdateChan)
	})

	It("emits metrics for dump sinks", func() {

		Expect(sinkManagerMetrics.Emit().Metrics[0].Name).To(Equal("numberOfDumpSinks"))
		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(0))

		sink = &dump.DumpSink{}
		sinkManagerMetrics.Inc(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(1))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(0))

		sinkManagerMetrics.Dec(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(0))
	})

	It("emits metrics for syslog sinks", func() {

		Expect(sinkManagerMetrics.Emit().Metrics[1].Name).To(Equal("numberOfSyslogSinks"))
		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(0))

		sink := &syslog.SyslogSink{}
		sinkManagerMetrics.Inc(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(1))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(0))

		sinkManagerMetrics.Dec(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(0))
	})

	It("emits metrics for websocket sinks", func() {

		Expect(sinkManagerMetrics.Emit().Metrics[2].Name).To(Equal("numberOfWebsocketSinks"))
		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(0))

		sink := &websocket.WebsocketSink{}
		sinkManagerMetrics.Inc(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(1))

		sinkManagerMetrics.Dec(sink)

		Expect(sinkManagerMetrics.Emit().Metrics[0].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[1].Value).To(Equal(0))
		Expect(sinkManagerMetrics.Emit().Metrics[2].Value).To(Equal(0))
	})

	It("emits metrics for firehose sinks", func() {
		Expect(sinkManagerMetrics.Emit().Metrics[3].Name).To(Equal("numberOfFirehoseSinks"))
		Expect(sinkManagerMetrics.Emit().Metrics[3].Value).To(Equal(0))

		sinkManagerMetrics.IncFirehose()
		Expect(sinkManagerMetrics.Emit().Metrics[3].Value).To(Equal(1))

		sinkManagerMetrics.DecFirehose()
		Expect(sinkManagerMetrics.Emit().Metrics[3].Value).To(Equal(0))

	})

	It("emits error counts for syslog sinks by app ID and drain URL", func() {
		sinkManagerMetrics.ReportSyslogError("app-id-1", "url-1")

		sinkManagerMetrics.ReportSyslogError("app-id-1", "url-2")
		sinkManagerMetrics.ReportSyslogError("app-id-1", "url-2")

		sinkManagerMetrics.ReportSyslogError("app-id-2", "url-3")

		drainErrorMetrics := sinkManagerMetrics.Emit().Metrics[4:7]
		Expect(drainErrorMetrics).To(ConsistOf(
			instrumentation.Metric{Name: "numberOfSyslogDrainErrors", Value: 1, Tags: map[string]interface{}{"appId": "app-id-1", "drainUrl": "url-1"}},
			instrumentation.Metric{Name: "numberOfSyslogDrainErrors", Value: 2, Tags: map[string]interface{}{"appId": "app-id-1", "drainUrl": "url-2"}},
			instrumentation.Metric{Name: "numberOfSyslogDrainErrors", Value: 1, Tags: map[string]interface{}{"appId": "app-id-2", "drainUrl": "url-3"}},
		))
	})

	It("emits dropped message counts by app id and drain url", func() {
		metrics := []sinks.Metric{sinks.Metric{Name: "numberOfMessagesLost", Tags: map[string]interface{}{"appId": "myApp"}, Value: 25}}
		sinkManagerMetrics.AddAppDrainMetrics(metrics)

		allMetrics := sinkManagerMetrics.Emit().Metrics
		lastAppDrainMetric := allMetrics[len(allMetrics)-1]
		Expect(lastAppDrainMetric).To(Equal(
			instrumentation.Metric{Name: "numberOfMessagesLost", Value: int64(25), Tags: map[string]interface{}{"appId": "myApp"}},
		))
	})

	It("emits the total number of message dropped", func() {
		dropUpdateChan <- 25
		dropUpdateChan <- 25

		totalDroppedMessageCountMetric := instrumentation.Metric{Name: "totalDroppedMessages", Value: int64(50)}

		Eventually(func() instrumentation.Metric { return sinkManagerMetrics.Emit().Metrics[4] }).Should(Equal(totalDroppedMessageCountMetric))
	})

	It("retains total dropped message count independently of current app drain metrics list", func() {
		dropUpdateChan <- 25
		dropUpdateChan <- 25

		metrics := []sinks.Metric{sinks.Metric{Name: "numberOfMessagesLost", Tags: map[string]interface{}{"appId": "myApp"}, Value: 378}}
		sinkManagerMetrics.AddAppDrainMetrics(metrics)

		var allMetrics []instrumentation.Metric
		Eventually(func() int64 {
			allMetrics = sinkManagerMetrics.Emit().Metrics
			totalMetric := allMetrics[4]

			return totalMetric.Value.(int64)
		}).Should(Equal(int64(50)))

		appMetric := allMetrics[5]
		Expect(appMetric.Value).To(Equal(int64(378)))
	})
})
