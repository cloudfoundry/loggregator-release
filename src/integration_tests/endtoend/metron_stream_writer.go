package endtoend
import (
	"time"
	"net"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/gomega"
)



type MetronStreamWriter struct {
	metronConn net.Conn
	numMessagesToSend int
	numMessagesSent int
	sleepTime time.Duration
}

func NewMetronStreamWriter(metronConn net.Conn, numMessagesSent int) *MetronStreamWriter{
	return &MetronStreamWriter{
		metronConn: metronConn,
		numMessagesToSend: numMessagesSent,
	}
}

func(w *MetronStreamWriter) Send(){
	if w.numMessagesSent >= w.numMessagesToSend {
		return
	}

	message := basicValueMessage(w.numMessagesSent)
	w.numMessagesSent += 1

	bytesOut, err := w.metronConn.Write(message)
	Expect(err).ToNot(HaveOccurred())
	Expect(bytesOut).To(Equal(len(message)))
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