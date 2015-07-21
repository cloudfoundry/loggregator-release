package experiment

import (
	"sync"
)

type MessageReader interface {
	Read()
}

type WriteStrategy interface {
	StartWriter()
}

type Experiment struct {
	writeStrategy WriteStrategy
	reader        MessageReader
	stopChan      chan struct{}
	writeRate     int
}

func NewExperiment(writeStrategy WriteStrategy, reader MessageReader, stopChan chan struct{}) *Experiment {
	return &Experiment{
		writeStrategy: writeStrategy,
		reader:        reader,
		stopChan:      stopChan,
	}
}

func (e *Experiment) startReader() {
	for {
		select {
		case <-e.stopChan:
			return
		default:
			e.reader.Read()
		}
	}
}

func (e *Experiment) Start() {
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		e.startReader()
	}()

	go func() {
		defer wg.Done()
		e.writeStrategy.StartWriter()
	}()

	wg.Wait()
}

func (e *Experiment) Stop() {
	close(e.stopChan)
}
