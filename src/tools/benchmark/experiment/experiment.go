package experiment

import (
	"log"
	"sync"
	"time"
)

type MessageReader interface {
	Read()
	Close()
}

type WriteStrategy interface {
	StartWriter()
	Stop()
}

type Experiment struct {
	writeStrategies []WriteStrategy
	reader          MessageReader
	stopChan        chan struct{}
	writeRate       int

	wg *sync.WaitGroup
}

func NewExperiment(reader MessageReader) *Experiment {
	return &Experiment{
		reader:   reader,
		stopChan: make(chan struct{}),
	}
}

func (e *Experiment) AddWriteStrategy(strategy WriteStrategy) {
	e.writeStrategies = append(e.writeStrategies, strategy)
}

func (e *Experiment) Warmup() {
	e.wg = &sync.WaitGroup{}

	e.wg.Add(1)
	reading := make(chan struct{})
	go func() {
		defer e.wg.Done()

		e.startReader(reading)
	}()

	for _, strategy := range e.writeStrategies {
		e.wg.Add(1)
		go func(s WriteStrategy) {
			defer e.wg.Done()
			s.StartWriter()
		}(strategy)
	}

	select {
	case <-reading:
		return
	case <-time.After(2 * time.Second):
		log.Fatalf("Failed to start reading")
	}
}

func (e *Experiment) Start() {
	e.wg.Wait()
}

func (e *Experiment) Stop() {
	for _, strategy := range e.writeStrategies {
		strategy.Stop()
	}
	// We cool down for a few seconds to drain all the final messages written by the writers
	time.Sleep(time.Second)
	close(e.stopChan)
}

func (e *Experiment) startReader(reading chan struct{}) {
	for {
		select {
		case <-e.stopChan:
			e.reader.Close()
			return
		default:
			e.reader.Read()
			if reading != nil {
				close(reading)
				reading = nil
			}
		}
	}
}
