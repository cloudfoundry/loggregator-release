package writestrategies

import (
	"math/rand"
	"time"
)

type ConstantWriteStrategy struct {
	generator MessageGenerator
	writer    MessageWriter
	writeRate int
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
	}
}

func (s *ConstantWriteStrategy) StartWriter(stopChan chan struct{}) {
	writeInterval := time.Second / time.Duration(s.writeRate)
	ticker := time.NewTicker(writeInterval)
	for {
		select {
		case <-stopChan:
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
	parameters BurstParameters
}

func NewBurstWriteStrategy(generator MessageGenerator, writer MessageWriter, params BurstParameters) *BurstWriteStrategy {
	return &BurstWriteStrategy{
		generator:  generator,
		writer:     writer,
		parameters: params,
	}
}

func (s *BurstWriteStrategy) StartWriter(stopChan chan struct{}) {
	ticker := time.NewTicker(s.parameters.Frequency)
	for {
		select {
		case <-stopChan:
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
