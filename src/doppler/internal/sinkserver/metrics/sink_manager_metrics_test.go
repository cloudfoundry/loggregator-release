package metrics_test

import (
	"doppler/internal/sinks"
	"doppler/internal/sinks/containermetric"
	"doppler/internal/sinks/dump"
	"doppler/internal/sinks/syslog"
	"doppler/internal/sinks/websocket"
	"doppler/internal/sinkserver/metrics"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	dropsondeMetrics "github.com/cloudfoundry/dropsonde/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SinkManagerMetrics", func() {
	var (
		fakeEventEmitter   = fake.NewFakeEventEmitter("doppler")
		sinkManagerMetrics *metrics.SinkManagerMetrics
		sink               sinks.Sink
	)

	BeforeSuite(func() {
		sender := metric_sender.NewMetricSender(fakeEventEmitter)
		batcher := metricbatcher.New(sender, 100*time.Millisecond)
		dropsondeMetrics.Initialize(sender, batcher)
	})

	BeforeEach(func() {
		fakeEventEmitter.Reset()
		sinkManagerMetrics = metrics.NewSinkManagerMetrics()
	})

	AfterEach(func() {
		sinkManagerMetrics.Stop()
	})

	It("emits metrics for dump sinks", func() {
		sinkManagerMetrics := metrics.NewSinkManagerMetrics()
		Eventually(fakeEventEmitter.GetMessages).Should(BeEmpty())

		sink = &dump.DumpSink{}
		sinkManagerMetrics.Inc(sink)

		expected := fake.Message{
			Origin: "doppler",
			Event: &events.ValueMetric{
				Name:  proto.String("messageRouter.numberOfDumpSinks"),
				Value: proto.Float64(1),
				Unit:  proto.String("sinks"),
			},
		}
		Eventually(fakeEventEmitter.GetMessages, 2).Should(ContainElement(expected))

		fakeEventEmitter.Reset()

		sinkManagerMetrics.Dec(sink)

		expected.Event = &events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfDumpSinks"),
			Value: proto.Float64(0),
			Unit:  proto.String("sinks"),
		}
		Eventually(fakeEventEmitter.GetMessages, 2).Should(ContainElement(expected))
	})

	It("emits metrics for syslog sinks", func() {
		Eventually(fakeEventEmitter.GetMessages).Should(BeEmpty())

		sink = &syslog.SyslogSink{}
		sinkManagerMetrics.Inc(sink)

		expected := fake.Message{
			Origin: "doppler",
			Event: &events.ValueMetric{
				Name:  proto.String("messageRouter.numberOfSyslogSinks"),
				Value: proto.Float64(1),
				Unit:  proto.String("sinks"),
			},
		}
		Eventually(fakeEventEmitter.GetMessages, 2).Should(ContainElement(expected))

		fakeEventEmitter.Reset()

		sinkManagerMetrics.Dec(sink)

		expected.Event = &events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfSyslogSinks"),
			Value: proto.Float64(0),
			Unit:  proto.String("sinks"),
		}
		Eventually(fakeEventEmitter.GetMessages, 2).Should(ContainElement(expected))
	})

	It("emits metrics for websocket sinks", func() {
		Eventually(fakeEventEmitter.GetMessages).Should(BeEmpty())

		sink = &websocket.WebsocketSink{}
		sinkManagerMetrics.Inc(sink)

		expected := fake.Message{
			Origin: "doppler",
			Event: &events.ValueMetric{
				Name:  proto.String("messageRouter.numberOfWebsocketSinks"),
				Value: proto.Float64(1),
				Unit:  proto.String("sinks"),
			},
		}
		Eventually(fakeEventEmitter.GetMessages, 2).Should(ContainElement(expected))

		fakeEventEmitter.Reset()

		sinkManagerMetrics.Dec(sink)

		expected.Event = &events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfWebsocketSinks"),
			Value: proto.Float64(0),
			Unit:  proto.String("sinks"),
		}
		Eventually(fakeEventEmitter.GetMessages, 2).Should(ContainElement(expected))
	})

	XIt("emits metrics for firehose sinks", func() {
		Eventually(fakeEventEmitter.GetMessages).Should(BeEmpty())

		sinkManagerMetrics.IncFirehose()

		expected := fake.Message{
			Origin: "doppler",
			Event: &events.ValueMetric{
				Name:  proto.String("messageRouter.numberOfFirehoseSinks"),
				Value: proto.Float64(1),
				Unit:  proto.String("sinks"),
			},
		}
		Eventually(fakeEventEmitter.GetMessages, 2).Should(ContainElement(expected))

		fakeEventEmitter.Reset()

		sinkManagerMetrics.DecFirehose()

		expected.Event = &events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfFirehoseSinks"),
			Value: proto.Float64(0),
			Unit:  proto.String("sinks"),
		}
		Eventually(fakeEventEmitter.GetMessages, 2).Should(ContainElement(expected))
	})

	It("emits metrics for container metric sinks", func() {
		Eventually(fakeEventEmitter.GetMessages).Should(BeEmpty())

		sink = &containermetric.ContainerMetricSink{}
		sinkManagerMetrics.Inc(sink)

		expected := fake.Message{
			Origin: "doppler",
			Event: &events.ValueMetric{
				Name:  proto.String("messageRouter.numberOfContainerMetricSinks"),
				Value: proto.Float64(1),
				Unit:  proto.String("sinks"),
			},
		}
		Eventually(fakeEventEmitter.GetMessages, 2).Should(ContainElement(expected))

		fakeEventEmitter.Reset()

		sinkManagerMetrics.Dec(sink)

		expected.Event = &events.ValueMetric{
			Name:  proto.String("messageRouter.numberOfContainerMetricSinks"),
			Value: proto.Float64(0),
			Unit:  proto.String("sinks"),
		}
		Eventually(fakeEventEmitter.GetMessages, 2).Should(ContainElement(expected))
	})
})
