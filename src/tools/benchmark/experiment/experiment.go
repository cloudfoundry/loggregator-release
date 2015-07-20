package experiment

import "time"

type MessageReader interface {
	Read()
}

type MessageWriter interface {
	Send()
}

type Experiment struct {
	writer    MessageWriter
	reader    MessageReader
	stopChan  chan struct{}
	writeRate int
}

func New(writer MessageWriter, reader MessageReader, writeRate int) *Experiment {
	return &Experiment{
		writer:    writer,
		writeRate: writeRate,
		reader:    reader,
		stopChan:  make(chan struct{}),
	}
}

func (e *Experiment) startWriter() {
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

func (e *Experiment) startReader() {
	for {
		select {
		case <-e.stopChan:
			return
		default:
		}

		e.reader.Read()
	}
}

func (e *Experiment) Start() {
	go e.startReader()
	e.startWriter()
}

func (e *Experiment) Stop() {
	close(e.stopChan)
}
