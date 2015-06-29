package messagewriter

import (
	"fmt"
	"net"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type messageWriter struct {
	reporter sentMessagesReporter
}

var metronInput net.Conn

type sentMessagesReporter interface {
	IncrementSentMessages()
}

func NewMessageWriter(reporter sentMessagesReporter) *messageWriter {

	var err error
	metronInput, err = net.Dial("udp", "localhost:51161")
	if err != nil {
		fmt.Printf("DIAL Error: %s\n", err.Error())
	}

	return &messageWriter{
		reporter: reporter,
	}
}

func (mw *messageWriter) Send() {
	_, err := metronInput.Write(basicValueMessage())
	if err != nil {
		fmt.Printf("SEND Error: %s\n", err.Error())
	} else {
		mw.reporter.IncrementSentMessages()
	}
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
