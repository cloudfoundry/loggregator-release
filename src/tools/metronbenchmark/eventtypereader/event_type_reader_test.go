package eventtypereader_test

import (
	"tools/benchmark/messagegenerator"
	"tools/benchmark/metricsreporter"
	"tools/metronbenchmark/eventtypereader"

	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const testOrigin = "test-origin"

var _ = Describe("EventTypeReader", func() {
	var (
		receivedCounter   *metricsreporter.Counter
		reader            *fakeReader
		valueMetricReader *eventtypereader.EventTypeReader
		eventType         events.Envelope_EventType
	)

	BeforeEach(func() {
		receivedCounter = metricsreporter.NewCounter("counter")
		reader = &fakeReader{}
	})

	JustBeforeEach(func() {
		valueMetricReader = eventtypereader.New(receivedCounter, reader, eventType, testOrigin)
	})

	Context("ValueMetrics", func() {
		BeforeEach(func() {
			eventType = events.Envelope_ValueMetric
		})

		It("should report value metrics", func() {
			reader.event = messagegenerator.BasicValueMetricEnvelope(testOrigin)

			valueMetricReader.Read()

			Expect(receivedCounter.GetValue()).To(BeEquivalentTo(1))
		})

		It("should not report log messages", func() {
			reader.event = messagegenerator.BasicLogMessageEnvelope(testOrigin, "appID")

			valueMetricReader.Read()

			Expect(receivedCounter.GetValue()).To(BeEquivalentTo(0))
		})

		It("should only report test value metrics", func() {
			reader.event = messagegenerator.BasicValueMetricEnvelope("some-other-origin")

			valueMetricReader.Read()

			Expect(receivedCounter.GetValue()).To(BeEquivalentTo(0))
		})
	})

	Context("LogMessages", func() {
		BeforeEach(func() {
			eventType = events.Envelope_LogMessage
		})

		It("should report log messages", func() {
			reader.event = messagegenerator.BasicLogMessageEnvelope(testOrigin, "appID")

			valueMetricReader.Read()

			Expect(receivedCounter.GetValue()).To(BeEquivalentTo(1))
		})

		It("should not report ", func() {
			reader.event = messagegenerator.BasicCounterEvent(testOrigin)

			valueMetricReader.Read()

			Expect(receivedCounter.GetValue()).To(BeEquivalentTo(0))
		})

		It("should only report test log messages", func() {
			reader.event = messagegenerator.BasicLogMessageEnvelope("some-other-origin", "appID")

			valueMetricReader.Read()

			Expect(receivedCounter.GetValue()).To(BeEquivalentTo(0))
		})
	})
})

type fakeReader struct {
	event *events.Envelope
}

func (f *fakeReader) Read() *events.Envelope {
	return f.event
}

func (f *fakeReader) Close() {
}
