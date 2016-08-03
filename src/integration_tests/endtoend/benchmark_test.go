package endtoend_test

import (
	"integration_tests"
	"integration_tests/endtoend"
	"time"
	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/writestrategies"

	. "github.com/onsi/ginkgo"
)

const benchDuration = 15 * time.Second

var _ = Describe("End to end benchmarks", func() {
	Context("with metron->doppler in UDP", func() {
		Measure("constant load", func(b Benchmarker) {
			etcdCleanup, etcdClientURL := integration_tests.SetupEtcd()
			defer etcdCleanup()
			metronCleanup, metronPort, metronReady := integration_tests.SetupMetron(etcdClientURL, "udp")
			defer metronCleanup()
			dopplerCleanup, dopplerOutgoingPort := integration_tests.SetupDoppler(etcdClientURL, metronPort)
			defer dopplerCleanup()
			trafficcontrollerCleanup, tcPort := integration_tests.SetupTrafficcontroller(etcdClientURL, dopplerOutgoingPort, metronPort)
			defer trafficcontrollerCleanup()
			metronReady()

			const writeRatePerSecond = 1000
			metronStreamWriter := endtoend.NewMetronStreamWriter(metronPort)
			firehoseReader := endtoend.NewFirehoseReader(tcPort)

			generator := messagegenerator.NewValueMetricGenerator()

			writeStrategy := writestrategies.NewConstantWriteStrategy(generator, metronStreamWriter, writeRatePerSecond)
			ex := experiment.NewExperiment(firehoseReader)
			ex.AddWriteStrategy(writeStrategy)

			ex.Warmup()

			go func() {
				time.Sleep(benchDuration)
				ex.Stop()
			}()
			b.Time("runtime", func() {
				ex.Start()
			})

			reportResults(firehoseReader, metronStreamWriter.Writes, b)
		}, 1)

		Measure("load that is bursting", func(b Benchmarker) {
			etcdCleanup, etcdClientURL := integration_tests.SetupEtcd()
			defer etcdCleanup()
			metronCleanup, metronPort, metronReady := integration_tests.SetupMetron(etcdClientURL, "udp")
			defer metronCleanup()
			dopplerCleanup, dopplerOutgoingPort := integration_tests.SetupDoppler(etcdClientURL, metronPort)
			defer dopplerCleanup()
			trafficcontrollerCleanup, tcPort := integration_tests.SetupTrafficcontroller(etcdClientURL, dopplerOutgoingPort, metronPort)
			defer trafficcontrollerCleanup()
			metronReady()

			metronStreamWriter := endtoend.NewMetronStreamWriter(metronPort)
			firehoseReader := endtoend.NewFirehoseReader(tcPort)

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

			go func() {
				time.Sleep(benchDuration)
				ex.Stop()
			}()
			b.Time("runtime", func() {
				ex.Start()
			})

			reportResults(firehoseReader, metronStreamWriter.Writes, b)
		}, 1)
	})

	Context("with metron->doppler in TCP", func() {
		Measure("constant load", func(b Benchmarker) {
			etcdCleanup, etcdClientURL := integration_tests.SetupEtcd()
			defer etcdCleanup()
			metronCleanup, metronPort, metronReady := integration_tests.SetupMetron(etcdClientURL, "tcp")
			defer metronCleanup()
			dopplerCleanup, dopplerOutgoingPort := integration_tests.SetupDoppler(etcdClientURL, metronPort)
			defer dopplerCleanup()
			trafficcontrollerCleanup, tcPort := integration_tests.SetupTrafficcontroller(etcdClientURL, dopplerOutgoingPort, metronPort)
			defer trafficcontrollerCleanup()
			metronReady()

			const writeRatePerSecond = 1000
			metronStreamWriter := endtoend.NewMetronStreamWriter(metronPort)
			firehoseReader := endtoend.NewFirehoseReader(tcPort)

			generator := messagegenerator.NewValueMetricGenerator()

			writeStrategy := writestrategies.NewConstantWriteStrategy(generator, metronStreamWriter, writeRatePerSecond)
			ex := experiment.NewExperiment(firehoseReader)
			ex.AddWriteStrategy(writeStrategy)

			ex.Warmup()

			go func() {
				time.Sleep(benchDuration)
				ex.Stop()
			}()
			b.Time("runtime", func() {
				ex.Start()
			})

			reportResults(firehoseReader, metronStreamWriter.Writes, b)
		}, 1)

		Measure("load that is bursting", func(b Benchmarker) {
			etcdCleanup, etcdClientURL := integration_tests.SetupEtcd()
			defer etcdCleanup()
			metronCleanup, metronPort, metronReady := integration_tests.SetupMetron(etcdClientURL, "tcp")
			defer metronCleanup()
			dopplerCleanup, dopplerOutgoingPort := integration_tests.SetupDoppler(etcdClientURL, metronPort)
			defer dopplerCleanup()
			trafficcontrollerCleanup, tcPort := integration_tests.SetupTrafficcontroller(etcdClientURL, dopplerOutgoingPort, metronPort)
			defer trafficcontrollerCleanup()
			metronReady()

			metronStreamWriter := endtoend.NewMetronStreamWriter(metronPort)
			firehoseReader := endtoend.NewFirehoseReader(tcPort)

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

			go func() {
				time.Sleep(benchDuration)
				ex.Stop()
			}()
			b.Time("runtime", func() {
				ex.Start()
			})

			reportResults(firehoseReader, metronStreamWriter.Writes, b)
		}, 1)
	})

	Context("with metron->doppler in TLS", func() {
		Measure("constant load", func(b Benchmarker) {
			etcdCleanup, etcdClientURL := integration_tests.SetupEtcd()
			defer etcdCleanup()
			metronCleanup, metronPort, metronReady := integration_tests.SetupMetron(etcdClientURL, "tls")
			defer metronCleanup()
			dopplerCleanup, dopplerOutgoingPort := integration_tests.SetupDoppler(etcdClientURL, metronPort)
			defer dopplerCleanup()
			trafficcontrollerCleanup, tcPort := integration_tests.SetupTrafficcontroller(etcdClientURL, dopplerOutgoingPort, metronPort)
			defer trafficcontrollerCleanup()
			metronReady()

			const writeRatePerSecond = 1000
			metronStreamWriter := endtoend.NewMetronStreamWriter(metronPort)
			firehoseReader := endtoend.NewFirehoseReader(tcPort)

			generator := messagegenerator.NewValueMetricGenerator()

			writeStrategy := writestrategies.NewConstantWriteStrategy(generator, metronStreamWriter, writeRatePerSecond)
			ex := experiment.NewExperiment(firehoseReader)
			ex.AddWriteStrategy(writeStrategy)

			ex.Warmup()

			go func() {
				time.Sleep(benchDuration)
				ex.Stop()
			}()
			b.Time("runtime", func() {
				ex.Start()
			})

			reportResults(firehoseReader, metronStreamWriter.Writes, b)
		}, 1)

		Measure("load that is bursting", func(b Benchmarker) {
			etcdCleanup, etcdClientURL := integration_tests.SetupEtcd()
			defer etcdCleanup()
			metronCleanup, metronPort, metronReady := integration_tests.SetupMetron(etcdClientURL, "tls")
			defer metronCleanup()
			dopplerCleanup, dopplerOutgoingPort := integration_tests.SetupDoppler(etcdClientURL, metronPort)
			defer dopplerCleanup()
			trafficcontrollerCleanup, tcPort := integration_tests.SetupTrafficcontroller(etcdClientURL, dopplerOutgoingPort, metronPort)
			defer trafficcontrollerCleanup()
			metronReady()

			metronStreamWriter := endtoend.NewMetronStreamWriter(metronPort)
			firehoseReader := endtoend.NewFirehoseReader(tcPort)

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

			go func() {
				time.Sleep(benchDuration)
				ex.Stop()
			}()
			b.Time("runtime", func() {
				ex.Start()
			})

			reportResults(firehoseReader, metronStreamWriter.Writes, b)
		}, 1)
	})
})

func reportResults(r *endtoend.FirehoseReader, written int, benchmarker Benchmarker) {
	var totalMessagesReceivedByFirehose float64
	totalMessagesReceivedByFirehose = r.TestMetricCount + r.NonTestMetricCount

	percentMessageLossBetweenMetronAndDoppler := computePercentLost(r.MetronSentMessageCount, r.DopplerReceivedMessageCount)
	percentMessageLossBetweenDopplerAndFirehose := computePercentLost(r.DopplerSentMessageCount, totalMessagesReceivedByFirehose)
	percentMessageLossOverEntirePipeline := computePercentLost(float64(written), r.TestMetricCount)

	benchmarker.RecordValue("Messages lost between metron and doppler", r.MetronSentMessageCount-r.DopplerReceivedMessageCount)
	benchmarker.RecordValue("Messages lost between doppler and firehose", r.DopplerSentMessageCount-totalMessagesReceivedByFirehose)
	benchmarker.RecordValue("Total message loss over entire pipeline", float64(written)-r.TestMetricCount)
	benchmarker.RecordValue("Message Loss percentage between Metron and Doppler ", percentMessageLossBetweenMetronAndDoppler)
	benchmarker.RecordValue("Message Loss percentage between Doppler and Firehose nozzle", percentMessageLossBetweenDopplerAndFirehose)
	benchmarker.RecordValue("Total message loss percentage (received by metron but not received by the firehose)", percentMessageLossOverEntirePipeline)
}

func computePercentLost(totalMessages, receivedMessages float64) float64 {
	return ((totalMessages - receivedMessages) / totalMessages) * 100
}
