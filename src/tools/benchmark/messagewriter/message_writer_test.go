package messagewriter_test

import (
	"tools/benchmark/messagewriter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"tools/benchmark/metricsreporter"
)

var _ = Describe("MessageWriter", func() {
	var sentCounter *metricsreporter.Counter
	BeforeEach(func() {
		sentCounter = metricsreporter.NewCounter()
	})

	It("should keep track of the total sent messages", func() {
		msgWriter := messagewriter.NewMessageWriter("localhost", 51161, "", sentCounter)
		message := []byte{}
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)
		msgWriter.Write(message)

		Expect(sentCounter.GetValue()).To(BeEquivalentTo(8))
	})

	It("should not increment total sent messages", func() {
		messagewriter.NewMessageWriter("localhost", 51161, "", sentCounter)
		Expect(sentCounter.GetValue()).To(BeEquivalentTo(0))
	})
})
