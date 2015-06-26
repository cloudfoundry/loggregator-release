package experiment

import (
	"time"
)

type Round interface {
	Perform(writer MessageWriter)
	Id() uint
	Rate() uint
	Duration() time.Duration
}

type round struct {
	id       uint
	rate     uint
	duration time.Duration
}

func NewRound(roundId uint, rate uint, duration time.Duration) Round {
	return &round{
		id:       roundId,
		rate:     rate,
		duration: duration,
	}
}

func (r *round) Perform(writer MessageWriter) {
	writeInterval := time.Second / time.Duration(r.rate)
	ticker := time.NewTicker(writeInterval)

	stopTime := time.Now().Add(r.duration)

	for t := range ticker.C {
		writer.Send(r.id, t)

		if t.After(stopTime) {
			return
		}
	}
}

func (r *round) Id() uint {
	return r.id
}

func (r *round) Rate() uint {
	return r.rate
}

func (r *round) Duration() time.Duration {
	return r.duration
}
