package containermetric_test

import (
	"doppler/sinks/containermetric"
	"time"

	"github.com/cloudfoundry/dropsonde/events"
	"github.com/gogo/protobuf/proto"

	"doppler/sinks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Containermetric", func() {
	var (
		sink              *containermetric.ContainerMetricSink
		eventChan         chan *events.Envelope
		updateMetricsChan = make(chan sinks.DrainMetric)
	)

	BeforeEach(func() {
		eventChan = make(chan *events.Envelope)

		sink = containermetric.NewContainerMetricSink("myApp", 2*time.Second, 2*time.Second, updateMetricsChan)
		go sink.Run(eventChan)
	})

	Describe("StreamId", func() {
		It("returns the application ID", func() {
			Expect(sink.StreamId()).To(Equal("myApp"))
		})
	})

	Describe("Run and GetLatest", func() {
		It("returns metrics for all instances", func() {
			now := time.Now()

			m1 := metricFor(1, now.Add(-1*time.Microsecond), 1, 1, 1)
			m2 := metricFor(2, now.Add(-1*time.Microsecond), 2, 2, 2)
			eventChan <- m1
			eventChan <- m2

			Eventually(sink.GetLatest).Should(HaveLen(2))

			metrics := sink.GetLatest()
			Expect(metrics).To(ConsistOf(m1, m2))
		})

		It("returns latest metric for an instance if it has a newer timestamp", func() {
			now := time.Now()

			m1 := metricFor(1, now.Add(-500*time.Microsecond), 1, 1, 1)
			eventChan <- m1

			Eventually(sink.GetLatest).Should(ConsistOf(m1))

			m2 := metricFor(1, now.Add(-200*time.Microsecond), 2, 2, 2)
			eventChan <- m2

			Eventually(sink.GetLatest).Should(ConsistOf(m2))
		})

		It("discards latest metric for an instance if it has an older timestamp", func() {
			now := time.Now()

			m1 := metricFor(1, now.Add(-1*time.Microsecond), 1, 1, 1)
			eventChan <- m1

			Eventually(sink.GetLatest).Should(ConsistOf(m1))

			m2 := metricFor(1, now.Add(-150*time.Microsecond), 2, 2, 2)
			eventChan <- m2

			Consistently(sink.GetLatest).Should(ConsistOf(m1))
		})

		It("ignores all other envelope types", func() {
			m1 := metricFor(1, time.Now().Add(-1*time.Microsecond), 1, 1, 1)
			eventChan <- m1

			Eventually(sink.GetLatest).Should(ConsistOf(m1))

			eventChan <- &events.Envelope{
				EventType: events.Envelope_LogMessage.Enum(),
			}

			Consistently(sink.GetLatest).Should(ConsistOf(m1))
		})

		It("removes the outdated container metrics", func() {
			m1 := metricFor(1, time.Now().Add(-1500*time.Millisecond), 1, 1, 1)
			eventChan <- m1

			Eventually(sink.GetLatest).Should(ConsistOf(m1))
			Eventually(sink.GetLatest).Should(BeEmpty())
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

	It("closes after a period of inactivity", func() {
		containerMetricSink := containermetric.NewContainerMetricSink("myAppId", 2*time.Second, 1*time.Millisecond, updateMetricsChan)
		containerMetricRunnerDone := make(chan struct{})
		inputChan := make(chan *events.Envelope)

		go func() {
			containerMetricSink.Run(inputChan)
			close(containerMetricRunnerDone)
		}()

		Eventually(containerMetricRunnerDone, 50*time.Millisecond).Should(BeClosed())
	})

	It("closes after input chan is closed", func() {
		containerMetricSink := containermetric.NewContainerMetricSink("myAppId", 2*time.Second, 10*time.Second, updateMetricsChan)
		containerMetricRunnerDone := make(chan struct{})
		inputChan := make(chan *events.Envelope)

		go func() {
			containerMetricSink.Run(inputChan)
			close(containerMetricRunnerDone)
		}()

		close(inputChan)

		Eventually(containerMetricRunnerDone, 50*time.Millisecond).Should(BeClosed())
	})

	It("resets the inactivity duration when a metric is received", func() {
		inactivityDuration := 1 * time.Millisecond
		containerMetricSink := containermetric.NewContainerMetricSink("myAppId", 2*time.Second, inactivityDuration, updateMetricsChan)
		containerMetricRunnerDone := make(chan struct{})
		inputChan := make(chan *events.Envelope)

		go func() {
			containerMetricSink.Run(inputChan)
			close(containerMetricRunnerDone)
		}()

		metric := metricFor(1, time.Now().Add(-1*time.Microsecond), 1, 1, 1)
		continuouslySend(inputChan, metric, 2*inactivityDuration)
		Expect(containerMetricRunnerDone).ShouldNot(BeClosed())
	})
	Describe("UpdateDroppedMessageCount", func() {
		It("returns number of dropped messages on metrics channel", func() {
			drainMetric := sinks.DrainMetric{AppId: "myApp", DrainURL: "containerMetricSink", DroppedMsgCount: uint64(10)}

			metric := retrieveDroppedMsgCountMetric(sink, updateMetricsChan, 10)
			Expect(*metric).To(Equal(drainMetric))
		})

		It("does not send message if droppedMsgCount is 0", func() {
			Expect(retrieveDroppedMsgCountMetric(sink, updateMetricsChan, 0)).To(BeNil())
		})
	})
})

func retrieveDroppedMsgCountMetric(sink sinks.Sink, updateMetricsChan chan sinks.DrainMetric, messageCount uint64) *sinks.DrainMetric {
	go sink.UpdateDroppedMessageCount(messageCount)

	var recvMetric *sinks.DrainMetric
	ticker := time.NewTicker(500 * time.Millisecond)
	select {
	case metric := <-updateMetricsChan:
		recvMetric = &metric
	case <-ticker.C:
	}
	return recvMetric
}

func continuouslySend(inputChan chan<- *events.Envelope, message *events.Envelope, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case inputChan <- message:
		case <-timer.C:
			return
		}
	}
}

func metricFor(instanceId int32, timestamp time.Time, cpu float64, mem uint64, disk uint64) *events.Envelope {
	unixTimestamp := timestamp.UnixNano()
	return &events.Envelope{
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(unixTimestamp),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("myApp"),
			InstanceIndex: proto.Int32(instanceId),
			CpuPercentage: proto.Float64(cpu),
			MemoryBytes:   proto.Uint64(mem),
			DiskBytes:     proto.Uint64(disk),
		},
	}
}
