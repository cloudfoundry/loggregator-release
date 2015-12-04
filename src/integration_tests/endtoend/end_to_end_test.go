package endtoend_test

import (
	"integration_tests/endtoend"
	"time"
	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/writestrategies"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	timeoutSeconds = 15
)

type Stopper interface {
	Stop()
}

var _ = Describe("End to end test", func() {
	benchmarkEndToEnd := func() {
		Measure("dropsonde metrics being passed from metron to the firehose nozzle", func(b Benchmarker) {
			const writeRatePerSecond = 1000
			metronStreamWriter := endtoend.NewMetronStreamWriter()

			firehoseReader := endtoend.NewFirehoseReader()

			generator := messagegenerator.NewValueMetricGenerator()

			writeStrategy := writestrategies.NewConstantWriteStrategy(generator, metronStreamWriter, writeRatePerSecond)
			ex := experiment.NewExperiment(firehoseReader)
			ex.AddWriteStrategy(writeStrategy)

			ex.Warmup()

			go stopExperimentAfterTimeout(ex)
			b.Time("runtime", func() {
				ex.Start()
			})

			reportResults(firehoseReader, metronStreamWriter.Writes, b)
			Expect(float64(firehoseReader.MetronSentMessageCount)).To(BeNumerically(">", writeRatePerSecond))
		}, 1)

		Measure("dropsonde metrics being passed from metron to the firehose nozzle in burst sequence", func(b Benchmarker) {
			metronStreamWriter := endtoend.NewMetronStreamWriter()
			firehoseReader := endtoend.NewFirehoseReader()

			params := writestrategies.BurstParameters{
				Minimum:   10,
				Maximum:   100,
				Frequency: time.Second,
			}

			generator := messagegenerator.NewValueMetricGenerator()
			writeStrategy := writestrategies.NewBurstWriteStrategy(generator, metronStreamWriter, params)
			ex := experiment.NewExperiment(firehoseReader)
			ex.AddWriteStrategy(writeStrategy)

			ex.Warmup()

			go stopExperimentAfterTimeout(ex)
			b.Time("runtime", func() {
				ex.Start()
			})

			reportResults(firehoseReader, metronStreamWriter.Writes, b)
		}, 1)
	}

	Context("UDP", func() {
		BeforeEach(func() {
			dopplerConfig = "dopplerudp"
		})

		benchmarkEndToEnd()
	})

	Context("TLS", func() {
		BeforeEach(func() {
			dopplerConfig = "dopplertls"
		})

		benchmarkEndToEnd()
	})
})

func reportResults(r *endtoend.FirehoseReader, written int, benchmarker Benchmarker) {
	var totalMessagesReceivedByFirehose float64
	totalMessagesReceivedByFirehose = r.TestMetricCount + r.NonTestMetricCount

	percentMessageLossBetweenMetronAndDoppler := computePercentLost(r.MetronSentMessageCount, r.DopplerReceivedMessageCount)
	percentMessageLossBetweenDopplerAndFirehose := computePercentLost(r.DopplerSentMessageCount, totalMessagesReceivedByFirehose)
	percentMessageLossOverEntirePipeline := computePercentLost(float64(written), r.TestMetricCount)

	benchmarker.RecordValue("Messages lost between metron and doppler", r.MetronSentMessageCount - r.DopplerReceivedMessageCount)
	benchmarker.RecordValue("Messages lost between doppler and firehose", r.DopplerSentMessageCount - totalMessagesReceivedByFirehose)
	benchmarker.RecordValue("Total message loss over entire pipeline", float64(written) - r.TestMetricCount)
	benchmarker.RecordValue("Message Loss percentage between Metron and Doppler ", percentMessageLossBetweenMetronAndDoppler)
	benchmarker.RecordValue("Message Loss percentage between Doppler and Firehose nozzle", percentMessageLossBetweenDopplerAndFirehose)
	benchmarker.RecordValue("Total message loss percentage (received by metron but not received by the firehose)", percentMessageLossOverEntirePipeline)

	Expect(percentMessageLossBetweenMetronAndDoppler).To(BeNumerically("<", 5.0))
	Expect(percentMessageLossBetweenMetronAndDoppler).To(BeNumerically(">", -1.0))
	Expect(percentMessageLossBetweenDopplerAndFirehose).To(BeNumerically("<", 5.0))
	Expect(percentMessageLossBetweenDopplerAndFirehose).To(BeNumerically(">", -1.0))
	Expect(percentMessageLossOverEntirePipeline).To(BeNumerically("<", 5.0))
	Expect(percentMessageLossOverEntirePipeline).To(BeNumerically(">", -1.0))
}

func computePercentLost(totalMessages, receivedMessages float64) float64 {
	return ((totalMessages - receivedMessages) / totalMessages) * 100
}

func stopExperimentAfterTimeout(ex Stopper) {
	time.Sleep(time.Duration(timeoutSeconds) * time.Second)
	ex.Stop()
}
