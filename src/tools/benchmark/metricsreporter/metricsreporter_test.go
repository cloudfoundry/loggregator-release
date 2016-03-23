package metricsreporter_test

import (
	"fmt"
	"tools/benchmark/metricsreporter"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("MetricsReporter", func() {
	var (
		buffer   *gbytes.Buffer
		reporter *metricsreporter.MetricsReporter
		start    time.Time
		counters []*metricsreporter.Counter
	)

	JustBeforeEach(func() {
		buffer = gbytes.NewBuffer()
		reporter = metricsreporter.New(time.Millisecond*10, buffer, counters...)
		start = time.Now()
		go reporter.Start()
	})

	AfterEach(func() {
		buffer.Close()
	})

	Describe("SentCounter", func() {
		It("returns a non-nil counter", func() {
			Expect(reporter.SentCounter()).NotTo(BeNil())
		})
	})

	Describe("ReceiveCounter", func() {
		It("returns a non-nil counter", func() {
			Expect(reporter.ReceivedCounter()).NotTo(BeNil())
		})
	})

	Describe("Rate", func() {
		It("reports rate", func() {
			reporter.SentCounter().IncrementValue()
			time.Sleep(time.Second)

			reporter.Stop()
			end := time.Now()

			expectedRate := float64(reporter.SentCounter().GetTotal()) / end.Sub(start).Seconds()
			Eventually(reporter.Rate()).Should(BeNumerically("~", expectedRate, 0.1))
		})
	})

	Describe("Duration", func() {
		It("reports test duration", func() {
			reporter.ReceivedCounter().IncrementValue()
			time.Sleep(time.Second)

			reporter.Stop()
			end := time.Now()

			Eventually(reporter.Duration()).Should(BeNumerically("~", end.Sub(start), time.Second))
		})
	})

	Describe("Output", func() {
		Context("with no additional counters", func() {

			It("increments sent counter", func() {
				reporter.SentCounter().IncrementValue()
				reporter.SentCounter().IncrementValue()
				Eventually(buffer).Should(gbytes.Say("2, 0"))
				reporter.Stop()
			})

			It("increments received counter", func() {
				reporter.ReceivedCounter().IncrementValue()
				Eventually(buffer).Should(gbytes.Say("0, 1"))
				reporter.Stop()
			})

			It("reports metric after reportTime is up", func() {
				reporter.SentCounter().IncrementValue()
				reporter.SentCounter().IncrementValue()
				reporter.ReceivedCounter().IncrementValue()

				Eventually(buffer).Should(gbytes.Say("2, 1"))
				Eventually(buffer).Should(gbytes.Say("0, 0"))
				reporter.Stop()
			})

			It("writes averages", func() {
				reporter.SentCounter().IncrementValue()
				reporter.SentCounter().IncrementValue()
				reporter.ReceivedCounter().IncrementValue()

				Eventually(reporter.NumTicks, "20ms", "1ms").Should(BeEquivalentTo(1))
				reporter.Stop()
				duration, rate := reporter.Duration(), reporter.Rate()
				Expect(reporter.NumTicks()).To(BeEquivalentTo(1), "The reporter is writing too quickly")

				Eventually(buffer).Should(gbytes.Say(fmt.Sprintf("Averages: %s, 2.00, 1.00, %.2f/s, 50.00%%", duration, rate)))
			})

			It("writes rate", func() {
				reporter.SentCounter().IncrementValue()
				reporter.ReceivedCounter().IncrementValue()

				Eventually(reporter.NumTicks, "20ms", "1ms").Should(BeEquivalentTo(1))
				rate := reporter.Rate()
				duration := reporter.Duration()
				reporter.Stop()

				Expect(reporter.NumTicks()).To(BeEquivalentTo(1), "The reporter is writing too quickly")

				Eventually(buffer).Should(gbytes.Say(fmt.Sprintf("%s, 1, 1, %.2f/s, 0.00%%", duration, rate)))
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
				counters = []*metricsreporter.Counter{counter1, counter2}
			})

			It("reports the column names", func() {
				Eventually(buffer).Should(gbytes.Say("Runtime, Sent, Received, Rate, PercentLoss, counter1, counter2"))
			})

			It("reports additional counter values", func() {
				reporter.ReceivedCounter().IncrementValue()
				reporter.SentCounter().IncrementValue()

				counter1.IncrementValue()
				counter2.IncrementValue()
				counter2.IncrementValue()

				Eventually(reporter.NumTicks, "20ms", "1ms").Should(BeEquivalentTo(1))

				Eventually(buffer, 10).Should(gbytes.Say(fmt.Sprintf("%s, 1, 1, %.2f/s, 0.00%%, 1, 2", reporter.Duration(), reporter.Rate())))
			})

			It("reports individual counter average", func() {
				reporter.SentCounter().IncrementValue()
				reporter.SentCounter().IncrementValue()
				reporter.ReceivedCounter().IncrementValue()

				counter1.IncrementValue()
				counter2.IncrementValue()
				counter2.IncrementValue()
				counter2.IncrementValue()

				Eventually(reporter.NumTicks, "20ms", "1ms").Should(BeEquivalentTo(1))

				reporter.SentCounter().IncrementValue()
				reporter.SentCounter().IncrementValue()
				reporter.ReceivedCounter().IncrementValue()

				counter1.IncrementValue()
				counter2.IncrementValue()
				counter2.IncrementValue()
				counter2.IncrementValue()

				Eventually(reporter.NumTicks, "20ms", "1ms").Should(BeEquivalentTo(2))
				reporter.Stop()
				Expect(reporter.NumTicks()).To(BeEquivalentTo(2), "The reporter is writing too quickly")

				Eventually(buffer).Should(gbytes.Say(fmt.Sprintf("Averages: %s, 2.00, 1.00, %.2f/s, 50.00%%, 1, 3", reporter.Duration(), reporter.Rate())))
			})
		})
	})
})
