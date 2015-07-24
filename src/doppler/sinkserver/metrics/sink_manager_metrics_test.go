package metrics_test

import (
	"doppler/sinks"
	"doppler/sinks/dump"
	"doppler/sinks/syslog"
	"doppler/sinks/websocket"
	"doppler/sinkserver/metrics"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SinkManagerMetrics", func() {

	var sinkManagerMetrics *metrics.SinkManagerMetrics
	var sink sinks.Sink

	BeforeEach(func() {
		fakeEventEmitter.Reset()
		sinkManagerMetrics = metrics.NewSinkManagerMetrics()
	})

	It("emits metrics for dump sinks", func() {
		Expect(fakeEventEmitter.GetMessages()).To(BeEmpty())

		sink = &dump.DumpSink{}
		sinkManagerMetrics.Inc(sink)

		Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfDumpSinks"),
			Value: proto.Float64(1),
			Unit:  proto.String("sinks"),
		}))

		sinkManagerMetrics.Dec(sink)

		Expect(fakeEventEmitter.GetMessages()[1].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfDumpSinks"),
			Value: proto.Float64(0),
			Unit:  proto.String("sinks"),
		}))
	})

	It("emits metrics for syslog sinks", func() {
		Expect(fakeEventEmitter.GetMessages()).To(BeEmpty())

		sink = &syslog.SyslogSink{}
		sinkManagerMetrics.Inc(sink)

		Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfSyslogSinks"),
			Value: proto.Float64(1),
			Unit:  proto.String("sinks"),
		}))

		sinkManagerMetrics.Dec(sink)

		Expect(fakeEventEmitter.GetMessages()[1].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfSyslogSinks"),
			Value: proto.Float64(0),
			Unit:  proto.String("sinks"),
		}))
	})

	It("emits metrics for websocket sinks", func() {
		Expect(fakeEventEmitter.GetMessages()).To(BeEmpty())

		sink = &websocket.WebsocketSink{}
		sinkManagerMetrics.Inc(sink)

		Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfWebsocketSinks"),
			Value: proto.Float64(1),
			Unit:  proto.String("sinks"),
		}))

		sinkManagerMetrics.Dec(sink)

		Expect(fakeEventEmitter.GetMessages()[1].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfWebsocketSinks"),
			Value: proto.Float64(0),
			Unit:  proto.String("sinks"),
		}))
	})

	It("emits metrics for firehose sinks", func() {
		Expect(fakeEventEmitter.GetMessages()).To(BeEmpty())

		sinkManagerMetrics.IncFirehose()
		Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfFirehoseSinks"),
			Value: proto.Float64(1),
			Unit:  proto.String("sinks"),
		}))

		sinkManagerMetrics.DecFirehose()
		Expect(fakeEventEmitter.GetMessages()[1].Event.(*events.ValueMetric)).To(Equal(&events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfFirehoseSinks"),
			Value: proto.Float64(0),
			Unit:  proto.String("sinks"),
		}))
	})

	It("updates dropped message count", func() {
		var delta int64 = 25
		sinkManagerMetrics.UpdateDroppedMessageCount(delta)

		Eventually(fakeEventEmitter.GetMessages).Should(HaveLen(1))
		Expect(fakeEventEmitter.GetMessages()[0].Event.(*events.CounterEvent)).To(Equal(&events.CounterEvent{
			Name:  proto.String("messageRouter.totalDroppedMessages"),
			Delta: proto.Uint64(25),
		}))
	})

})
