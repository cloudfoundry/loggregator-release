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

	It("emits the total number of message dropped", func() {
		dropUpdateChan <- 25
		dropUpdateChan <- 25

		totalDroppedMessageCountMetric := instrumentation.Metric{Name: "totalDroppedMessages", Value: int64(50)}

		Eventually(func() instrumentation.Metric { return sinkManagerMetrics.Emit().Metrics[4] }).Should(Equal(totalDroppedMessageCountMetric))
	})
})
