package experiment_test

import (
	"tools/metronbenchmark/experiment"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

			Eventually(func() int { return reader.readCount }).Should(BeNumerically(">", 0))
			Eventually(func() int { return writer.writeCount }).Should(BeNumerically(">", 0))
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
	writeCount int
	output     chan struct{}
}

func NewFakeWriter(outputChan chan struct{}) *fakeWriter {
	return &fakeWriter{
		output: outputChan,
	}
}

func (writer *fakeWriter) Send() {
	writer.writeCount++
	writer.output <- struct{}{}
}

type fakeReader struct {
	readCount int
	input     chan struct{}
}

func NewFakeReader(inputChan chan struct{}) *fakeReader {
	return &fakeReader{
		input: inputChan,
	}
}

func (reader *fakeReader) Read() {
	<-reader.input
	reader.readCount++
}

type fakeReporter struct {
	totalSent int
}

func (f *fakeReporter) IncrementSentMessages() {
	f.totalSent++
}
