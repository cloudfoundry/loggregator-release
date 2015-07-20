package endtoend

import (
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"net"
)

type MetronStreamWriter struct {
	metronConn      net.Conn
	numMessagesSent int
}

func NewMetronStreamWriter() *MetronStreamWriter {
	metronConn, err := net.Dial("udp", "localhost:49625")
	if err != nil {
		panic(err)
	}
	return &MetronStreamWriter{metronConn: metronConn}
}

func (w *MetronStreamWriter) Send() {
	message := basicValueMessage(w.numMessagesSent)
	w.numMessagesSent += 1

	_, err := w.metronConn.Write(message)
	if err != nil {
		panic(err)
	}
}

func basicValueMessage(i int) []byte {
	message, _ := proto.Marshal(basicValueMessageEnvelope(i))
	return message
}

func basicValueMessageEnvelope(i int) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("fake-origin"),
		EventType: events.Envelope_ValueMetric.Enum(),
		ValueMetric: &events.ValueMetric{
			Name:  proto.String("fake-metric-name"),
			Value: proto.Float64(float64(i)),
			Unit:  proto.String("fake-unit"),
		},
	}
}
