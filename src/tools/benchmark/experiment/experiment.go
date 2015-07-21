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
	writeStrategies []WriteStrategy
	reader        MessageReader
	stopChan      chan struct{}
	writeRate     int
}

func NewExperiment(reader MessageReader, stopChan chan struct{}) *Experiment {
	return &Experiment{
		reader:        reader,
		stopChan:      stopChan,
	}
}

func (e *Experiment) AddWriteStrategy(strategy WriteStrategy) {
	e.writeStrategies = append(e.writeStrategies, strategy)
}

func (e *Experiment) Start() {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		e.startReader()
	}()

	for _, strategy := range e.writeStrategies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			strategy.StartWriter()
		}()
	}

	wg.Wait()
}

func (e *Experiment) Stop() {
	close(e.stopChan)
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
