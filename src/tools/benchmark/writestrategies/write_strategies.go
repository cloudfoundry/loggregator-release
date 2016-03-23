package writestrategies

import (
	"math/rand"
	"sync"
	"time"
)

type ConstantWriteStrategy struct {
	generator MessageGenerator
	writer    MessageWriter
	writeRate int
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

type MessageGenerator interface {
	Generate() []byte
}

type MessageWriter interface {
	Write([]byte)
}

func NewConstantWriteStrategy(generator MessageGenerator, writer MessageWriter, writeRate int) *ConstantWriteStrategy {
	return &ConstantWriteStrategy{
		generator: generator,
		writer:    writer,
		writeRate: writeRate,
		stopChan:  make(chan struct{}),
	}
}

func (s *ConstantWriteStrategy) StartWriter() {
	writeInterval := time.Second / time.Duration(s.writeRate)
	ticker := time.NewTicker(writeInterval)
	for {
		select {
		case <-s.stopChan:
			s.wg.Done()
			return
		case <-ticker.C:
			s.writer.Write(s.generator.Generate())
		}
	}
}

func (s *ConstantWriteStrategy) Stop() {
	s.wg.Add(1)
	close(s.stopChan)
	s.wg.Wait()
}

type BurstParameters struct {
	Minimum   int
	Maximum   int
	Frequency time.Duration
}

type BurstWriteStrategy struct {
	generator  MessageGenerator
	writer     MessageWriter
	parameters BurstParameters
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

func NewBurstWriteStrategy(generator MessageGenerator, writer MessageWriter, params BurstParameters) *BurstWriteStrategy {
	return &BurstWriteStrategy{
		generator:  generator,
		writer:     writer,
		parameters: params,
		stopChan:   make(chan struct{}),
	}
}

func (s *BurstWriteStrategy) StartWriter() {
	ticker := time.NewTicker(s.parameters.Frequency)
	for {
		select {
		case <-s.stopChan:
			s.wg.Done()
			return
		case <-ticker.C:
			burst := random(s.parameters.Minimum, s.parameters.Maximum)
			for i := 0; i < burst; i++ {
				s.writer.Write(s.generator.Generate())
			}
		}
	}
}

func (s *BurstWriteStrategy) Stop() {
	s.wg.Add(1)
	close(s.stopChan)
	s.wg.Wait()
}

func random(minimum int, maximum int) int {
	if minimum == maximum {
		return minimum
	}
	rand.Seed(time.Now().Unix())
	diff := maximum - minimum
	return minimum + rand.Intn(diff)
}
