package metricsreporter_test

import (
	"tools/benchmark/metricsreporter"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Metricsreporter", func() {
	var buffer *gbytes.Buffer
	var reporter *metricsreporter.MetricsReporter

	BeforeEach(func() {
		buffer = gbytes.NewBuffer()
		reporter = metricsreporter.New(time.Millisecond*10, buffer)
		go reporter.Start()
	})

	AfterEach(func() {
		buffer.Close()
	})

	It("should increment sent counter", func() {
		reporter.GetSentCounter().IncrementValue()
		reporter.GetSentCounter().IncrementValue()
		Eventually(buffer).Should(gbytes.Say("2, 0"))
		reporter.Stop()
	})

	It("should increment received counter", func() {
		reporter.GetReceivedCounter().IncrementValue()
		Eventually(buffer).Should(gbytes.Say("0, 1"))
		reporter.Stop()
	})

	It("should report metric after reportTime is up", func() {
		reporter.GetSentCounter().IncrementValue()
		reporter.GetSentCounter().IncrementValue()
		reporter.GetReceivedCounter().IncrementValue()

		Eventually(buffer).Should(gbytes.Say("2, 1"))
		Eventually(buffer).Should(gbytes.Say("0, 0"))
		reporter.Stop()
	})

	It("should write averages", func() {
		reporter.GetSentCounter().IncrementValue()
		reporter.GetSentCounter().IncrementValue()
		reporter.GetReceivedCounter().IncrementValue()

		Eventually(reporter.GetNumTicks, "20ms", "1ms").Should(Equal(int32(1)))
		reporter.Stop()
		Expect(reporter.GetNumTicks()).To(Equal(int32(1)), "The reporter is writing too quickly")

		Eventually(buffer).Should(gbytes.Say("Averages: 2, 1, 50%"))
	})
})
