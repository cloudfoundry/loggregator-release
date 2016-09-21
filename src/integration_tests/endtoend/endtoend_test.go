package endtoend_test

import (
	"time"

	"integration_tests"
	"integration_tests/endtoend"
	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/writestrategies"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("End to end tests", func() {
	Context("with metron and doppler in tcp mode", func() {
		It("sends messages", func() {
			etcdCleanup, etcdClientURL := integration_tests.SetupEtcd()
			defer etcdCleanup()
			metronCleanup, metronPort, metronReady := integration_tests.SetupMetron(etcdClientURL, "udp")
			defer metronCleanup()
			dopplerCleanup, dopplerWSPort, dopplerGRPCPort := integration_tests.SetupDoppler(etcdClientURL, metronPort)
			defer dopplerCleanup()
			trafficcontrollerCleanup, tcPort := integration_tests.SetupTrafficcontroller(etcdClientURL, dopplerWSPort, dopplerGRPCPort, metronPort)
			defer trafficcontrollerCleanup()
			metronReady()

			const writeRatePerSecond = 10
			metronStreamWriter := endtoend.NewMetronStreamWriter(metronPort)
			generator := messagegenerator.NewLogMessageGenerator("custom-app-id")
			writeStrategy := writestrategies.NewConstantWriteStrategy(generator, metronStreamWriter, writeRatePerSecond)

			firehoseReader := endtoend.NewFirehoseReader(tcPort)
			ex := experiment.NewExperiment(firehoseReader)
			ex.AddWriteStrategy(writeStrategy)

			ex.Warmup()
			go func() {
				defer ex.Stop()
				time.Sleep(2 * time.Second)
			}()
			ex.Start()

			Eventually(firehoseReader.LogMessages).Should(Receive(ContainSubstring("custom-app-id")))
		}, 10)
	})
})
