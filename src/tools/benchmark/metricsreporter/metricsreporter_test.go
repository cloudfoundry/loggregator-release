package metricsreporter_test

import (
	"tools/benchmark/metricsreporter"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("MetricsReporter", func() {
	var buffer *gbytes.Buffer
	var reporter *metricsreporter.MetricsReporter

	Context("with no additional counters", func() {
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

			Eventually(reporter.GetNumTicks, "20ms", "1ms").Should(BeEquivalentTo(1))
			reporter.Stop()
			Expect(reporter.GetNumTicks()).To(BeEquivalentTo(1), "The reporter is writing too quickly")

			Eventually(buffer).Should(gbytes.Say("Averages: 2, 1, 50%"))
		})
	})

	Context("with additional counters", func() {
		var (
			counter1 *metricsreporter.Counter
			counter2 *metricsreporter.Counter
		)

		BeforeEach(func() {
			counter1 = metricsreporter.NewCounter("counter1")
			counter2 = metricsreporter.NewCounter("counter2")

			buffer = gbytes.NewBuffer()
			reporter = metricsreporter.New(time.Millisecond*10, buffer, counter1, counter2)
			go reporter.Start()
		})

		AfterEach(func() {
			buffer.Close()
		})

		It("should report the column names", func() {
			Eventually(buffer).Should(gbytes.Say("Sent, Received, PercentLoss, counter1, counter2"))
		})

		It("should report additional counter values", func() {
			reporter.GetReceivedCounter().IncrementValue()
			reporter.GetSentCounter().IncrementValue()

			counter1.IncrementValue()
			counter2.IncrementValue()
			counter2.IncrementValue()

			Eventually(buffer).Should(gbytes.Say("1, 1, 0%, 1, 2"))
		})

		It("should report individual counter average", func() {
			reporter.GetSentCounter().IncrementValue()
			reporter.GetSentCounter().IncrementValue()
			reporter.GetReceivedCounter().IncrementValue()

			counter1.IncrementValue()
			counter2.IncrementValue()
			counter2.IncrementValue()
			counter2.IncrementValue()

			Eventually(reporter.GetNumTicks, "20ms", "1ms").Should(BeEquivalentTo(1))

			reporter.GetSentCounter().IncrementValue()
			reporter.GetSentCounter().IncrementValue()
			reporter.GetReceivedCounter().IncrementValue()

			counter1.IncrementValue()
			counter2.IncrementValue()
			counter2.IncrementValue()
			counter2.IncrementValue()

			Eventually(reporter.GetNumTicks, "20ms", "1ms").Should(BeEquivalentTo(2))

			reporter.Stop()
			Expect(reporter.GetNumTicks()).To(BeEquivalentTo(2), "The reporter is writing too quickly")

			Eventually(buffer).Should(gbytes.Say("Averages: 2, 1, 50%, 1, 3"))
		})
	})
})
