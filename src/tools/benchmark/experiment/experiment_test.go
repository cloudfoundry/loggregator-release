package experiment_test

import (
	"tools/benchmark/experiment"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync/atomic"
)

var _ = Describe("Experiment", func() {
	var e *experiment.Experiment
	var writer *fakeWriter
	var reader *fakeReader

	BeforeEach(func() {
		inOutChan := make(chan struct{})
		reader = NewFakeReader(inOutChan)
		writer = NewFakeWriter(inOutChan)
		e = experiment.New(writer, reader, 1000)
	})

	Describe("Start", func() {
		It("sends and receives messages", func() {
			defer e.Stop()
			go e.Start()

			Eventually(func() int32 { return atomic.LoadInt32(&reader.readCount) }).Should(BeNumerically(">", 0))
			Eventually(func() int32 { return atomic.LoadInt32(&writer.writeCount) }).Should(BeNumerically(">", 0))
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

type fakeWriter struct {
	writeCount int32
	output     chan struct{}
}

func NewFakeWriter(outputChan chan struct{}) *fakeWriter {
	return &fakeWriter{
		output: outputChan,
	}
}

func (writer *fakeWriter) Send() {
	atomic.AddInt32(&writer.writeCount, 1)
	writer.output <- struct{}{}
}

type fakeReader struct {
	readCount int32
	input     chan struct{}
}

func NewFakeReader(inputChan chan struct{}) *fakeReader {
	return &fakeReader{
		input: inputChan,
	}
}

func (reader *fakeReader) Read() {
	<-reader.input
	atomic.AddInt32(&reader.readCount, 1)
}

type fakeReporter struct{}

func (f *fakeReporter) IncrementSentMessages() {}
