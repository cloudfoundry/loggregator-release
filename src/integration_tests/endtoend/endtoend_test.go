package endtoend_test

import (
	"integration_tests/endtoend"
	"time"

	"integration_tests"
	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/writestrategies"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("End to end tests", func() {
	It("sends messages from metron through doppler and traffic controller", func() {
		etcdCleanup, etcdClientURL := integration_tests.SetupEtcd()
		defer etcdCleanup()

		dopplerCleanup, dopplerWSPort, dopplerGRPCPort := integration_tests.StartTestDoppler(
			integration_tests.BuildTestDopplerConfig(etcdClientURL, 0),
		)
		defer dopplerCleanup()

		metronCleanup, metronPort, metronReady := integration_tests.SetupMetron("localhost", dopplerGRPCPort, 0)
		defer metronCleanup()
		trafficcontrollerCleanup, tcPort := integration_tests.SetupTrafficcontroller(
			etcdClientURL,
			dopplerWSPort,
			dopplerGRPCPort,
			metronPort,
		)
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
