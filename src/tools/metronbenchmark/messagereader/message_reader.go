package messagereader

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
)

type RoundResult struct {
	Received uint
	Latency  time.Duration
}

type LoggregatorReader interface {
	Read() (*events.LogMessage, bool)
}

type messageReader struct {
	lr      LoggregatorReader
	results map[uint]*RoundResult
	l       sync.RWMutex
}

func NewMessageReader(lr LoggregatorReader) *messageReader {
	return &messageReader{
		lr:      lr,
		results: make(map[uint]*RoundResult),
	}
}

func (mr *messageReader) Start() {
	msg, ok := mr.lr.Read()
	for ; ok; msg, ok = mr.lr.Read() {
		msgTxt := string(msg.Message)
		values := strings.Split(msgTxt, ":")
		roundId := parseUint(values[1])
		sequenceNum := parseUint(values[2])
		timestamp := time.Unix(0, parseInt(values[3]))
		mr.add(roundId, sequenceNum, timestamp)
	}
}

func (mr *messageReader) GetRoundResult(roundId uint) *RoundResult {
	mr.l.RLock()
	defer mr.l.RUnlock()

	return mr.results[roundId]
}

func (mr *messageReader) add(roundId uint, sequenceNum uint, timestamp time.Time) {
	mr.l.Lock()
	defer mr.l.Unlock()

	_, ok := mr.results[roundId]
	if !ok {
		mr.results[roundId] = &RoundResult{}
	}
	mr.results[roundId].Received++

	latency := time.Now().Sub(timestamp)
	if latency > mr.results[roundId].Latency {
		mr.results[roundId].Latency = latency
	}
}

func parseUint(v string) uint {
	r, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		panic(err)
	}
	return uint(r)
}

func parseInt(v string) int64 {
	r, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		panic(err)
	}
	return r
}
