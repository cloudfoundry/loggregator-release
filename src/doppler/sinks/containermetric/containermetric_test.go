package containermetric_test

import (
	"code.google.com/p/gogoprotobuf/proto"
	"doppler/sinks/containermetric"
	"github.com/cloudfoundry/dropsonde/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Containermetric", func() {
	var (
		sink      *containermetric.ContainerMetricSink
		eventChan chan *events.Envelope
	)

	BeforeEach(func() {
		eventChan = make(chan *events.Envelope)

		sink = containermetric.NewContainerMetricSink("myApp")
		go sink.Run(eventChan)
	})

	Describe("StreamId", func() {
		It("returns the application ID", func() {
			Expect(sink.StreamId()).To(Equal("myApp"))
		})
	})

	Describe("Run and GetLatest", func() {
		It("returns metrics for all instances", func() {
			m1 := metricFor(1, 1, 1, 1, 1)
			m2 := metricFor(2, 2, 2, 2, 2)
			eventChan <- m1
			eventChan <- m2

			Eventually(sink.GetLatest).Should(HaveLen(2))

			metrics := sink.GetLatest()
			Expect(metrics).To(ConsistOf(m1, m2))
		})

		It("returns latest metric for an instance if it has a newer timestamp", func() {
			m1 := metricFor(1, 1, 1, 1, 1)
			eventChan <- m1

			Eventually(sink.GetLatest).Should(ConsistOf(m1))

			m2 := metricFor(1, 2, 2, 2, 2)
			eventChan <- m2

			Eventually(sink.GetLatest).Should(ConsistOf(m2))
		})

		It("discards latest metric for an instance if it has an older timestamp", func() {
			m1 := metricFor(1, 2, 1, 1, 1)
			eventChan <- m1

			Eventually(sink.GetLatest).Should(ConsistOf(m1))

			m2 := metricFor(1, 1, 2, 2, 2)
			eventChan <- m2

			Consistently(sink.GetLatest).Should(ConsistOf(m1))
		})

		It("ignores all other envelope types", func() {
			m1 := metricFor(1, 1, 1, 1, 1)
			eventChan <- m1

			Eventually(sink.GetLatest).Should(ConsistOf(m1))

			eventChan <- &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
			}

			Consistently(sink.GetLatest).Should(ConsistOf(m1))

		})
	})

	Describe("Identifier", func() {
		It("returns 'container-metrics-' plus the application ID", func() {
			Expect(sink.Identifier()).To(Equal("container-metrics-myApp"))
		})

	})

	Describe("ShouldReceiveErrors", func() {
		It("returns false", func() {
			Expect(sink.ShouldReceiveErrors()).To(BeFalse())
		})
	})
})

func metricFor(instanceId int32, timestamp int64, cpu float64, mem uint64, disk uint64) *events.Envelope {
	return &events.Envelope{
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(timestamp),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("myApp"),
			InstanceIndex: proto.Int32(instanceId),
			CpuPercentage: proto.Float64(cpu),
			MemoryBytes:   proto.Uint64(mem),
			DiskBytes:     proto.Uint64(disk),
		},
	}
}
