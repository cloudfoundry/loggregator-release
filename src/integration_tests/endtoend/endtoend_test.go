package endtoend_test

import (
	"integration_tests/endtoend"
	"time"

	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/writestrategies"

	"testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("End to end tests", func() {
	It("sends messages from metron through doppler and traffic controller", func() {
		etcdCleanup, etcdClientURL := testservers.StartTestEtcd()
		defer etcdCleanup()

		dopplerCleanup, dopplerWSPort, dopplerGRPCPort := testservers.StartDoppler(
			testservers.BuildDopplerConfig(etcdClientURL, 0),
		)
		defer dopplerCleanup()

		metronCleanup, metronConfig, metronReady := testservers.StartMetron(
			testservers.BuildMetronConfig("localhost", dopplerGRPCPort, 0),
		)
		defer metronCleanup()
		trafficcontrollerCleanup, tcPort := testservers.StartTrafficController(
			testservers.BuildTrafficControllerConf(
				etcdClientURL,
				dopplerWSPort,
				dopplerGRPCPort,
				metronConfig.IncomingUDPPort,
			),
		)
		defer trafficcontrollerCleanup()
		metronReady()

		const writeRatePerSecond = 10
		metronStreamWriter := endtoend.NewMetronStreamWriter(metronConfig.IncomingUDPPort)
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
