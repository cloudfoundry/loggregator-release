package legacyreader_test

import (
	"github.com/cloudfoundry/sonde-go/events"
	"tools/benchmark/messagegenerator"
	"tools/legacymetronbenchmark/legacyreader"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LegacyReader", func() {
	Context("Read", func() {
		var (
			reporter     fakeReporter
			reader       fakeReader
			legacyReader *legacyreader.LegacyReader
		)

		BeforeEach(func() {
			reporter = fakeReporter{}
			reader = fakeReader{}
			legacyReader = legacyreader.NewLegacyReader(&reporter, &reader)
		})

		It("should report legacy log messages", func() {
			reader.event = messagegenerator.BasicLogMessageEnvelope("legacy")

			legacyReader.Read()

			Expect(reporter.count).To(Equal(1))
		})

		It("should report value metrics", func() {
			reader.event = messagegenerator.BasicValueMetricEnvelope("test-origin")

			legacyReader.Read()

			Expect(reporter.count).To(Equal(1))
		})

		It("should only count test log messages", func() {
			reader.event = messagegenerator.BasicLogMessageEnvelope("non-test-origin")

			legacyReader.Read()

			Expect(reporter.count).To(Equal(0))
		})

		It("should only count test value metrics", func() {
			reader.event = messagegenerator.BasicValueMetricEnvelope("non-test-origin")

			legacyReader.Read()

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
