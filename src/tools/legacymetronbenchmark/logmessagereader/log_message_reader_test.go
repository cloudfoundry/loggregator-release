package logmessagereader_test

import (
	"github.com/cloudfoundry/sonde-go/events"
	"tools/benchmark/messagegenerator"
	"tools/legacymetronbenchmark/logmessagereader"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogMessageReader", func() {
	Context("Read", func() {
		var (
			reporter  fakeReporter
			reader    fakeReader
			logReader *logmessagereader.LogMessageReader
		)

		BeforeEach(func() {
			reporter = fakeReporter{}
			reader = fakeReader{}
			logReader = logmessagereader.NewLogMessageReader(&reporter, &reader)
		})

		It("should report log messages", func() {
			reader.event = messagegenerator.BasicLogMessageEnvelope()

			logReader.Read()

			Expect(reporter.count).To(Equal(1))
		})

		It("should not report value metrics", func() {
			reader.event = messagegenerator.BasicValueMetricEnvelope()

			logReader.Read()

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
