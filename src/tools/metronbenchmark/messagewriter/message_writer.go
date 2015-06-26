package messagewriter

import (
	"fmt"
	"io"
	"net"
	"time"
	"github.com/gogo/protobuf/proto"
	"github.com/cloudfoundry/sonde-go/events"
)

type messageWriter struct {
	w io.Writer
	*roundSequencer
}

var metronInput net.Conn

func NewMessageWriter(w io.Writer) *messageWriter {

	var err error
	metronInput, err = net.Dial("udp", "localhost:51161")
	if err != nil {
		fmt.Printf("Error: %s", err.Error())
	}

	return &messageWriter{
		w:              w,
		roundSequencer: newRoundSequener(),
	}
}

func (mw *messageWriter) Send(roundId uint, timestamp time.Time) {
	fmt.Println("Sending Message...")
//	_, err := metronInput.Write(formatMsg(roundId, mw.nextSequence(roundId), timestamp))
	_, err := metronInput.Write(basicValueMessage())
	if err != nil {
		fmt.Printf("Error: %s", err.Error())
	}
	mw.w.Write(formatMsg(roundId, mw.nextSequence(roundId), timestamp))
}

func basicValueMessage() []byte {
	message, _ := proto.Marshal(basicValueMessageEnvelope())
	return message
}

func basicValueMessageEnvelope() *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("fake-metric-name"),
			Value: proto.Float64(42),
			Unit:  proto.String("fake-unit"),
		},
	}
}