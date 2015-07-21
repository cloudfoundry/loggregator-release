package writestrategies

import (
	"math/rand"
	"time"
)

type ConstantWriteStrategy struct {
	generator MessageGenerator
	writer    MessageWriter
	stopChan  chan struct{}
	writeRate int
}

type MessageGenerator interface {
	Generate() []byte
}

type MessageWriter interface {
	Write([]byte)
}

func NewConstantWriteStrategy(generator MessageGenerator, writer MessageWriter, writeRate int, stopChan chan struct{}) *ConstantWriteStrategy {
	return &ConstantWriteStrategy{
		generator: generator,
		writer:    writer,
		writeRate: writeRate,
		stopChan:  stopChan,
	}
}

func (s *ConstantWriteStrategy) StartWriter() {
	writeInterval := time.Second / time.Duration(s.writeRate)
	ticker := time.NewTicker(writeInterval)
	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.writer.Write(s.generator.Generate())
		}
	}
}

type BurstParameters struct {
	Minimum   int
	Maximum   int
	Frequency time.Duration
}

type BurstWriteStrategy struct {
	generator  MessageGenerator
	writer     MessageWriter
	stopChan   chan struct{}
	parameters BurstParameters
}

func NewBurstWriteStrategy(generator MessageGenerator, writer MessageWriter, params BurstParameters, stopChan chan struct{}) *BurstWriteStrategy {
	return &BurstWriteStrategy{
		generator:  generator,
		writer:     writer,
		parameters: params,
		stopChan:   stopChan,
	}
}

func (s *BurstWriteStrategy) StartWriter() {
	ticker := time.NewTicker(s.parameters.Frequency)
	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			burst := random(s.parameters.Minimum, s.parameters.Maximum)
			for i := 0; i < burst; i++ {
				s.writer.Write(s.generator.Generate())
			}
		}
	}
}

func random(minimum int, maximum int) int {
	rand.Seed(time.Now().Unix())
	diff := maximum - minimum
	return minimum + rand.Intn(diff)
}
