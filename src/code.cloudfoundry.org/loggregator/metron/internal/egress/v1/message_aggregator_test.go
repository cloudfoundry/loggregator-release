package v1_test

import (
	"time"

	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v1"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MessageAggregator", func() {
	var (
		mockWriter        *MockEnvelopeWriter
		messageAggregator *egress.MessageAggregator
		originalTTL       time.Duration
	)

	BeforeEach(func() {
		mockWriter = &MockEnvelopeWriter{}
		messageAggregator = egress.NewAggregator(
			mockWriter,
		)
		originalTTL = egress.MaxTTL
	})

	AfterEach(func() {
		egress.MaxTTL = originalTTL
	})

	It("passes value messages through", func() {
		inputMessage := createValueMessage()
		messageAggregator.Write(inputMessage)

		Expect(mockWriter.Events).To(HaveLen(1))
		Expect(mockWriter.Events[0]).To(Equal(inputMessage))
	})

	It("handles concurrent writes without data race", func() {
		inputMessage := createValueMessage()
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; i < 100; i++ {
				messageAggregator.Write(inputMessage)
			}
		}()
		for i := 0; i < 100; i++ {
			messageAggregator.Write(inputMessage)
		}
		<-done
	})

	Describe("counter processing", func() {
		It("sets the Total field on a CounterEvent ", func() {
			messageAggregator.Write(createCounterMessage("total", "fake-origin-4", nil))

			Expect(mockWriter.Events).To(HaveLen(1))
			outputMessage := mockWriter.Events[0]
			Expect(outputMessage.GetEventType()).To(Equal(events.Envelope_CounterEvent))
			expectCorrectCounterNameDeltaAndTotal(outputMessage, "total", 4, 4)
		})

		It("accumulates Deltas for CounterEvents with the same name, origin, and tags", func() {
			messageAggregator.Write(createCounterMessage(
				"total",
				"fake-origin-4",
				map[string]string{
					"protocol": "tcp",
				},
			))
			messageAggregator.Write(createCounterMessage(
				"total",
				"fake-origin-4",
				map[string]string{
					"protocol": "tcp",
				},
			))
			messageAggregator.Write(createCounterMessage(
				"total",
				"fake-origin-4",
				map[string]string{
					"protocol": "tcp",
				},
			))

			Expect(mockWriter.Events).To(HaveLen(3))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[0], "total", 4, 4)
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[1], "total", 4, 8)
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[2], "total", 4, 12)
		})

		It("accumulates differently-named counters separately", func() {
			messageAggregator.Write(createCounterMessage("total1", "fake-origin-4", nil))
			messageAggregator.Write(createCounterMessage("total2", "fake-origin-4", nil))

			Expect(mockWriter.Events).To(HaveLen(2))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[0], "total1", 4, 4)
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[1], "total2", 4, 4)
		})

		It("accumulates differently-tagged counters separately", func() {
			By("writing protocol tagged counters")
			messageAggregator.Write(createCounterMessage(
				"total",
				"fake-origin-4",
				map[string]string{
					"protocol": "grpc",
				},
			))
			messageAggregator.Write(createCounterMessage(
				"total",
				"fake-origin-4",
				map[string]string{
					"protocol": "tcp",
				},
			))
			messageAggregator.Write(createCounterMessage(
				"total",
				"fake-origin-4",
				map[string]string{
					"protocol": "grpc",
				},
			))

			By("writing counters tagged with key/value strings split differently")
			messageAggregator.Write(createCounterMessage(
				"total",
				"fake-origin-4",
				map[string]string{
					"proto": "other",
				},
			))

			Expect(mockWriter.Events).To(HaveLen(4))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[0], "total", 4, 4)
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[1], "total", 4, 4)
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[2], "total", 4, 8)
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[3], "total", 4, 4)
		})

		It("does not accumulate for counters when receiving a non-counter event", func() {
			messageAggregator.Write(createValueMessage())
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-4", nil))

			Expect(mockWriter.Events).To(HaveLen(2))
			Expect(mockWriter.Events[0].GetEventType()).To(Equal(events.Envelope_ValueMetric))
			Expect(mockWriter.Events[1].GetEventType()).To(Equal(events.Envelope_CounterEvent))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[1], "counter1", 4, 4)
		})

		It("accumulates independently for different origins", func() {
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-4", nil))
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-5", nil))
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-4", nil))

			Expect(mockWriter.Events).To(HaveLen(3))

			Expect(mockWriter.Events[0].GetOrigin()).To(Equal("fake-origin-4"))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[0], "counter1", 4, 4)

			Expect(mockWriter.Events[1].GetOrigin()).To(Equal("fake-origin-5"))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[1], "counter1", 4, 4)

			Expect(mockWriter.Events[2].GetOrigin()).To(Equal("fake-origin-4"))
			expectCorrectCounterNameDeltaAndTotal(mockWriter.Events[2], "counter1", 4, 8)
		})
	})

	Context("metrics", func() {
		var (
			fakeSender  *fake.FakeMetricSender
			mockBatcher *mockMetricBatcher
		)

		BeforeEach(func() {
			fakeSender = fake.NewFakeMetricSender()
			mockBatcher = newMockMetricBatcher()
			metrics.Initialize(fakeSender, mockBatcher)
		})

		It("emits a counter for counter events", func() {
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-1", nil))
			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("MessageAggregator.counterEventReceived"),
			))

			// since we're counting counters, let's make sure we're not adding their deltas
			messageAggregator.Write(createCounterMessage("counter1", "fake-origin-1", nil))
			Eventually(mockBatcher.BatchIncrementCounterInput).Should(BeCalled(
				With("MessageAggregator.counterEventReceived"),
			))
		})
	})
})

func createValueMessage() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("fake-metric-name"),
			Value: proto.Float64(42),
			Unit:  proto.String("fake-unit"),
		},
	}
}

func createCounterMessage(name, origin string, tags map[string]string) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String(origin),
		EventType: events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String(name),
			Delta: proto.Uint64(4),
		},
		Tags: tags,
	}
}

func expectCorrectCounterNameDeltaAndTotal(outputMessage *events.Envelope, name string, delta uint64, total uint64) {
	Expect(outputMessage.GetCounterEvent().GetName()).To(Equal(name))
	Expect(outputMessage.GetCounterEvent().GetDelta()).To(Equal(delta))
	Expect(outputMessage.GetCounterEvent().GetTotal()).To(Equal(total))
}
