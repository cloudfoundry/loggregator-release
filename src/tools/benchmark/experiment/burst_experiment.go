package experiment

import (
	"math/rand"
	"sync"
	"time"
)

type BurstParameters struct {
	Minimum   int
	Maximum   int
	Frequency time.Duration
}

type BurstExperiment struct {
	parameters BurstParameters
	writer     MessageWriter
	reader     MessageReader
	stopChan   chan struct{}
}

func NewBurstExperiment(writer MessageWriter, reader MessageReader, params BurstParameters) *BurstExperiment {
	return &BurstExperiment{
		parameters: params,
		writer:     writer,
		reader:     reader,
		stopChan:   make(chan struct{}),
	}
}

func (b *BurstExperiment) Start() {
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		b.startReader()
	}()

	go func() {
		defer wg.Done()
		b.startWriter()
	}()

	wg.Wait()
}

func (b *BurstExperiment) Stop() {
	close(b.stopChan)
}

func (b *BurstExperiment) startWriter() {
	ticker := time.NewTicker(b.parameters.Frequency)
	for {
		select {
		case <-b.stopChan:
			return
		case <-ticker.C:
			burst := random(b.parameters.Minimum, b.parameters.Maximum)
			for i := 0; i < burst; i++ {
				b.writer.Send()
			}
		}
	}
}

func (b *BurstExperiment) startReader() {
	for {
		select {
		case <-b.stopChan:
			return
		default:
			b.reader.Read()
		}
	}
}

func random(minimum int, maximum int) int {
	rand.Seed(time.Now().Unix())
	diff := maximum - minimum
	return minimum + rand.Intn(diff)
}
