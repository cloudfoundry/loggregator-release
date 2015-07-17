package experiment

import (
	"sync"
	"time"
)

type MessageGenerator interface {
	Generate() []byte
}

type MessageWriter interface {
	Write([]byte)
}

type MessageReader interface {
	Read()
}

type ConstantRateExperiment struct {
	generator MessageGenerator
	writer    MessageWriter
	reader    MessageReader
	stopChan  chan struct{}
	writeRate int
}

func NewConstantRateExperiment(generator MessageGenerator, writer MessageWriter, reader MessageReader, writeRate int) *ConstantRateExperiment {
	return &ConstantRateExperiment{
		generator: generator,
		writer:    writer,
		writeRate: writeRate,
		reader:    reader,
		stopChan:  make(chan struct{}),
	}
}

func (e *ConstantRateExperiment) startWriter() {
	writeInterval := time.Second / time.Duration(e.writeRate)
	ticker := time.NewTicker(writeInterval)
	for {
		select {
		case <-e.stopChan:
			return
		case <-ticker.C:
			e.writer.Write(e.generator.Generate())
		}
	}
}

func (e *ConstantRateExperiment) startReader() {
	for {
		select {
		case <-e.stopChan:
			return
		default:
			e.reader.Read()
		}
	}
}

func (e *ConstantRateExperiment) Start() {
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		e.startReader()
	}()

	go func() {
		defer wg.Done()
		e.startWriter()
	}()

	wg.Wait()
}

func (e *ConstantRateExperiment) Stop() {
	close(e.stopChan)
}
