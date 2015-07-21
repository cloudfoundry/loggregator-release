package experiment_test

import (
	"tools/benchmark/experiment"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync/atomic"
)

var _ = Describe("Experiment", func() {
	var e *experiment.Experiment
	var reader *fakeOtherReader
	var strategy *fakeWriteStrategy

	BeforeEach(func() {
		reader = &fakeOtherReader{}

		strategy = &fakeWriteStrategy{}
		stopChan := make(chan struct{})
		e = experiment.NewExperiment(reader, stopChan)
		e.AddWriteStrategy(strategy)
	})

	Describe("Start", func() {
		It("sends and receives messages", func() {
			defer e.Stop()
			go e.Start()

			Eventually(strategy.Started).Should(BeTrue())
			Eventually(reader.ReadCount).Should(BeNumerically(">", 0))
		})

		It("stops when we close the stop channel", func() {
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

type fakeWriteStrategy struct {
	started bool
}

func (s *fakeWriteStrategy) StartWriter() {
	s.started = true
}

func (s *fakeWriteStrategy) Started() bool {
	return s.started
}

type fakeOtherReader struct {
	readCount int32
}

func (reader *fakeOtherReader) Read() {
	atomic.AddInt32(&reader.readCount, 1)
}

func (reader *fakeOtherReader) ReadCount() int32 {
	return atomic.LoadInt32(&reader.readCount)
}
