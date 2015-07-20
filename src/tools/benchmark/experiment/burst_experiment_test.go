package experiment_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
	"tools/benchmark/experiment"
	"tools/benchmark/messagegenerator"
)

var _ = Describe("BurstExperiment", func() {
	var e *experiment.BurstExperiment
	var writer *fakeWriter
	var reader *fakeReader
	var params experiment.BurstParameters

	BeforeEach(func() {
		inOutChan := make(chan struct{})
		reader = NewFakeReader(inOutChan)
		writer = NewFakeWriter(inOutChan)
		params = experiment.BurstParameters{
			Minimum:   10,
			Maximum:   100,
			Frequency: 900 * time.Millisecond,
		}
		generator := messagegenerator.NewValueMetricGenerator()
		e = experiment.NewBurstExperiment(generator, writer, reader, params)
	})

	Describe("start", func() {
		It("sends and receives messages with the provided burst parameters", func() {
			defer e.Stop()
			go e.Start()

			Eventually(writer.WriteCount).Should(BeNumerically(">", params.Minimum))
			Eventually(writer.WriteCount).Should(BeNumerically("<", params.Maximum))

			Eventually(reader.ReadCount).Should(BeNumerically(">", params.Minimum))
			Eventually(reader.ReadCount).Should(BeNumerically("<", params.Maximum))
		})

		It("stops when we close the stop chanel", func() {
			doneChan := make(chan struct{})

			go func() {
				e.Start()
				close(doneChan)
			}()

			e.Stop()
			Eventually(doneChan).Should(BeClosed())
		})
	})
})
