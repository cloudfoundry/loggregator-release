package messagewriter

import (
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/proto"
)

type channelLogWriter struct {
	c chan<- *events.LogMessage
	*roundSequencer
}

func NewChannelLogWriter(c chan<- *events.LogMessage) *channelLogWriter {
	return &channelLogWriter{
		c:              c,
		roundSequencer: newRoundSequener(),
	}
}

func (w *channelLogWriter) Send(roundId uint, timestamp time.Time) {
	lm := &events.LogMessage{
		Message:     formatMsg(roundId, w.nextSequence(roundId), timestamp),
		MessageType: events.LogMessage_OUT.Enum(),
		Timestamp:   proto.Int64(time.Now().UnixNano()),
	}
	w.c <- lm
}
