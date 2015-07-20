package experiment

import (
	"sync"
	"time"
)

type MessageReader interface {
	Read()
}

type MessageWriter interface {
	Send()
}

type ConstantRateExperiment struct {
	writer    MessageWriter
	reader    MessageReader
	stopChan  chan struct{}
	writeRate int
}

func NewConstantRateExperiment(writer MessageWriter, reader MessageReader, writeRate int) *ConstantRateExperiment {
	return &ConstantRateExperiment{
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
			e.writer.Send()
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
