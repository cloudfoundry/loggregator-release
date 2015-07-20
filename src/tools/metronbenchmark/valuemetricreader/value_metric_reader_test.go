package valuemetricreader_test

import (
	"github.com/cloudfoundry/sonde-go/events"
	"tools/benchmark/messagegenerator"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"tools/metronbenchmark/valuemetricreader"
)

var _ = Describe("ValueMetricReader", func() {
	Context("Read", func() {
		var (
			reporter          fakeReporter
			reader            fakeReader
			valueMetricReader *valuemetricreader.ValueMetricReader
		)

		BeforeEach(func() {
			reporter = fakeReporter{}
			reader = fakeReader{}
			valueMetricReader = valuemetricreader.NewValueMetricReader(&reporter, &reader)
		})

		It("should report value metrics", func() {
			reader.event = messagegenerator.BasicValueMetricEnvelope()

			valueMetricReader.Read()

			Expect(reporter.count).To(Equal(1))
		})

		It("should not report log messages", func() {
			reader.event = messagegenerator.BasicLogMessageEnvelope()

			valueMetricReader.Read()

			Expect(reporter.count).To(Equal(0))
		})
	})
})

type fakeReporter struct {
	count int
}

func (f *fakeReporter) IncrementReceivedMessages() {
	f.count++
}

type fakeReader struct {
	event *events.Envelope
}

func (f *fakeReader) Read() *events.Envelope {
	return f.event
}
