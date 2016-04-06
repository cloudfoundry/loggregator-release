package endtoend_test

import (
	"integration_tests/endtoend"
	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
	"tools/benchmark/writestrategies"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("End to end tests", func() {
	Context("with metron sending on tcp", func() {
		BeforeEach(func() {
			metronConfig = "metrontcp"
		})

		Context("with doppler reading on tcp", func() {
			BeforeEach(func() {
				dopplerConfig = "dopplertcp"
			})

			It("can send a message", func() {
				const writeRatePerSecond = 1000
				metronStreamWriter := endtoend.NewMetronStreamWriter()
				firehoseReader := endtoend.NewFirehoseReader()
				generator := messagegenerator.NewLogMessageGenerator("custom-app-id")

				writeStrategy := writestrategies.NewConstantWriteStrategy(generator, metronStreamWriter, writeRatePerSecond)
				ex := experiment.NewExperiment(firehoseReader)
				ex.AddWriteStrategy(writeStrategy)

				ex.Warmup()

				go stopExperimentAfterTimeout(ex)
				ex.Start()

				Eventually(firehoseReader.LastLogMessage()).Should(ContainSubstring("custom-app-id"))
			}, 10)
		})
	})
})
