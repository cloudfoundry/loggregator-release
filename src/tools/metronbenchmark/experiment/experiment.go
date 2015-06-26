package experiment

import (
	"time"
	"tools/metronbenchmark/messagereader"
)

type MessageReader interface {
	Start()
	GetRoundResult(roundId uint) *messagereader.RoundResult
}

type MessageWriter interface {
	Send(roundId uint, timestamp time.Time)
	GetSentCount(roundId uint) uint
}

type Experiment struct {
	writer   MessageWriter
	reader   MessageReader
	newRound RoundFactory
	rounds   []Round
}

type RoundFactory func(id uint, rate uint, duration time.Duration) Round

func New(writer MessageWriter, reader MessageReader, newRound RoundFactory) *Experiment {
	return &Experiment{
		writer:   writer,
		reader:   reader,
		newRound: newRound,
	}
}

func (e *Experiment) Start(stop <-chan struct{}) {
	e.rounds = []Round{}
	for roundId := uint(1); ; roundId++ {
		select {
		case <-stop:
			return
		default:
		}

		rate := (uint(1) << (roundId - 1)) * 100

		round := e.newRound(roundId, rate, 10*time.Second)
		e.rounds = append(e.rounds, round)
		round.Perform(e.writer)
	}
}

func (e *Experiment) Results() Results {
	results := []Result{}

	for _, round := range e.rounds {
		sent := e.writer.GetSentCount(round.Id())
		rcvd := uint(0)

		roundResult := e.reader.GetRoundResult(round.Id())
		if roundResult != nil {
			rcvd = roundResult.Received
		}

		lossRate := 100 * (float64(sent-rcvd) / float64(sent))
		results = append(results, Result{
			MessageRate:      round.Rate(),
			MessagesSent:     sent,
			MessagesReceived: rcvd,
			LossPercentage:   lossRate,
		})
	}

	return results
}
