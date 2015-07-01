package metricsreporter_test

import (
	"tools/metronbenchmark/metricsreporter"

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
		reporter = metricsreporter.New(time.Millisecond, buffer)
		go reporter.Start()
	})

	AfterEach(func() {
		buffer.Close()
		reporter.Stop()
	})

	It("should increment sent counter", func() {
		reporter.IncrementSentMessages()
		reporter.IncrementSentMessages()
		Eventually(buffer).Should(gbytes.Say("2, 0"))
	})

	It("should increment received counter", func() {
		reporter.IncrementReceivedMessages()
		Eventually(buffer).Should(gbytes.Say("0, 1"))
	})

	It("should report metric after reportTime is up", func() {
		reporter.IncrementSentMessages()
		reporter.IncrementSentMessages()
		reporter.IncrementReceivedMessages()

		Eventually(buffer).Should(gbytes.Say("2, 1"))
		Eventually(buffer).Should(gbytes.Say("0, 0"))
	})
})
