package messagewriter

import "sync"

type roundSequencer struct {
	results map[uint]uint //[roundId]count
	l       sync.RWMutex
}

func newRoundSequener() *roundSequencer {
	return &roundSequencer{
		results: make(map[uint]uint),
	}
}

func (rs *roundSequencer) GetSentCount(roundId uint) uint {
	rs.l.RLock()
	defer rs.l.RUnlock()

	return rs.results[roundId]
}

func (rs *roundSequencer) nextSequence(roundId uint) uint {
	rs.l.Lock()
	defer rs.l.Unlock()

	rs.results[roundId]++
	return rs.results[roundId]
}
