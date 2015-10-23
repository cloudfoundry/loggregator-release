package experiment_test

import (
	"tools/benchmark/experiment"

	"sync/atomic"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Experiment", func() {
	var e *experiment.Experiment
	var reader *fakeOtherReader
	var strategy *fakeWriteStrategy

	BeforeEach(func() {
		reader = &fakeOtherReader{}

		strategy = &fakeWriteStrategy{}
		e = experiment.NewExperiment(reader)
		e.AddWriteStrategy(strategy)
	})

	Describe("Warmup & Start", func() {
		It("sends and receives messages", func() {
			defer e.Stop()
			e.Warmup()
			go e.Start()

			Eventually(strategy.Started).Should(BeTrue())
			Eventually(reader.ReadCount).Should(BeNumerically(">", 0))
		})

		It("stops when we close the stop channel", func() {
			doneChan := make(chan struct{})

			e.Warmup()
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
	started int32
}

func (s *fakeWriteStrategy) StartWriter() {
	atomic.StoreInt32(&s.started, 1)
}

func (s *fakeWriteStrategy) Stop() {

}

func (s *fakeWriteStrategy) Started() bool {
	return atomic.LoadInt32(&s.started) == 1
}

type fakeOtherReader struct {
	readCount int32
}

func (reader *fakeOtherReader) Read() {
	atomic.AddInt32(&reader.readCount, 1)
}

func (reader *fakeOtherReader) Close() {
}

func (reader *fakeOtherReader) ReadCount() int32 {
	return atomic.LoadInt32(&reader.readCount)
}
