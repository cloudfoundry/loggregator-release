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
			const numMessagesSent = 1000
			metronStreamWriter := endtoend.NewMetronStreamWriter()

			firehoseReader := endtoend.NewFirehoseReader()

			generator := messagegenerator.NewValueMetricGenerator()

			writeStrategy := writestrategies.NewConstantWriteStrategy(generator, metronStreamWriter, numMessagesSent)
			ex := experiment.NewExperiment(firehoseReader)
			ex.AddWriteStrategy(writeStrategy)

			ex.Warmup()

			go stopExperimentAfterTimeout(ex)
			b.Time("runtime", func() {
				ex.Start()
			})

			reportResults(firehoseReader, b)
			Expect(float64(firehoseReader.MetronMessageCount)).To(BeNumerically(">", numMessagesSent))
		}, 3)

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

			reportResults(firehoseReader, b)
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

func reportResults(r *endtoend.FirehoseReader, benchmarker Benchmarker) {
	var totalMessagesReceivedByFirehose float64
	totalMessagesReceivedByFirehose = r.TestMetricCount + r.NonTestMetricCount

	percentMessageLossBetweenMetronAndDoppler := computePercentLost(r.MetronMessageCount, r.DopplerMessageCount)
	percentMessageLossBetweenDopplerAndFirehose := computePercentLost(r.DopplerMessageCount, totalMessagesReceivedByFirehose)
	percentMessageLossOverEntirePipeline := computePercentLost(r.MetronMessageCount, totalMessagesReceivedByFirehose)

	benchmarker.RecordValue("Messages received by metron", r.MetronMessageCount)
	benchmarker.RecordValue("Messages received by doppler", r.DopplerMessageCount)
	benchmarker.RecordValue("Test messages received by firehose nozzle", r.TestMetricCount)
	benchmarker.RecordValue("Non-test messages received by firehose nozzle", r.NonTestMetricCount)
	benchmarker.RecordValue("Total messages received by firehose nozzle", totalMessagesReceivedByFirehose)
	benchmarker.RecordValue("Message Loss percentage between Metron and Doppler ", percentMessageLossBetweenMetronAndDoppler)
	benchmarker.RecordValue("Message Loss percentage between Doppler and Firehose nozzle", percentMessageLossBetweenDopplerAndFirehose)
	benchmarker.RecordValue("Total message loss percentage (received by metron but not received by the firehose)", percentMessageLossOverEntirePipeline)

	Expect(percentMessageLossBetweenMetronAndDoppler).To(BeNumerically("<", 5.0))
	Expect(percentMessageLossBetweenDopplerAndFirehose).To(BeNumerically("<", 5.0))
	Expect(percentMessageLossOverEntirePipeline).To(BeNumerically("<", 5.0))
}

func computePercentLost(totalMessages, receivedMessages float64) float64 {
	return ((totalMessages - receivedMessages) / totalMessages) * 100
}

func stopExperimentAfterTimeout(ex Stopper) {
	t := time.NewTicker(time.Duration(timeoutSeconds) * time.Second)
	<-t.C
	ex.Stop()
}
