package valuemetricreader_test

import (
	"tools/benchmark/messagegenerator"

	"github.com/cloudfoundry/sonde-go/events"

	"tools/benchmark/metricsreporter"
	"tools/metronbenchmark/valuemetricreader"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValueMetricReader", func() {
	Context("Read", func() {
		var (
			receivedCounter   *metricsreporter.Counter
			reader            fakeReader
			valueMetricReader *valuemetricreader.ValueMetricReader
		)

		BeforeEach(func() {
			receivedCounter = metricsreporter.NewCounter("counter")
			reader = fakeReader{}
			valueMetricReader = valuemetricreader.NewValueMetricReader(receivedCounter, &reader)
		})

		It("should report value metrics", func() {
			reader.event = messagegenerator.BasicValueMetricEnvelope(valuemetricreader.TestOrigin)

			valueMetricReader.Read()

			Expect(receivedCounter.GetValue()).To(BeEquivalentTo(1))
		})

		It("should not report log messages", func() {
			reader.event = messagegenerator.BasicLogMessageEnvelope(valuemetricreader.TestOrigin, "appID")

			valueMetricReader.Read()

			Expect(receivedCounter.GetValue()).To(BeEquivalentTo(0))
		})

		It("should only report test value metrics", func() {
			reader.event = messagegenerator.BasicValueMetricEnvelope("some-other-origin")

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
