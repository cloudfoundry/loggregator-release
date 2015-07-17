package messagewriter_test

import (
	"tools/benchmark/messagewriter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MessageWriter", func() {
	var reporter *fakeReporter
	BeforeEach(func() {
		reporter = &fakeReporter{}
	})

	It("should keep track of the total sent messages", func() {
		msgWriter := messagewriter.NewMessageWriter("localhost", 51161, "", reporter)
		message := []byte{}
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)

		Expect(reporter.totalSent).To(Equal(8))
	})

	It("should not increment total sent messages", func() {
		messagewriter.NewMessageWriter("localhost", 51161, "", reporter)
		Expect(reporter.totalSent).To(Equal(0))
	})
})

type fakeReporter struct {
	totalSent int
}

func (f *fakeReporter) IncrementSentMessages() {
	f.totalSent++
}
