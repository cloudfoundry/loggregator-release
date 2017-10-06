package sinks_test

import (
	"code.cloudfoundry.org/loggregator/doppler/internal/sinks"
	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = XDescribe("SinkManagerMetrics", func() {
	var (
		fakeEventEmitter   = fake.NewFakeEventEmitter("doppler")
		sinkManagerMetrics *sinks.SinkManagerMetrics
		sink               sinks.Sink
	)

	BeforeEach(func() {
		fakeEventEmitter.Reset()
		sinkManagerMetrics = sinks.NewSinkManagerMetrics()
	})

	AfterEach(func() {
		sinkManagerMetrics.Stop()
	})

	// TODO remove dependency in test on global metrics package
	XIt("emits metrics for dump sinks", func() {
		sinkManagerMetrics := sinks.NewSinkManagerMetrics()
		Eventually(fakeEventEmitter.GetMessages).Should(BeEmpty())

		sink = &sinks.DumpSink{}
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

	// TODO remove dependency in test on global metrics package
	XIt("emits metrics for syslog sinks", func() {
		Eventually(fakeEventEmitter.GetMessages).Should(BeEmpty())

		sink = &sinks.SyslogSink{}
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

	// TODO remove dependency in test on global metrics package
	XIt("emits metrics for container metric sinks", func() {
		Eventually(fakeEventEmitter.GetMessages).Should(BeEmpty())

		sink = &sinks.ContainerMetricSink{}
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
