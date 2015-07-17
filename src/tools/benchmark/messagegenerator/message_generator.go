package messagegenerator
import (
	"github.com/gogo/protobuf/proto"
	"github.com/cloudfoundry/sonde-go/events"
)

type ValueMetricGenerator struct{}

func NewValueMetricGenerator() *ValueMetricGenerator{
	return &ValueMetricGenerator{}
}

func (*ValueMetricGenerator) Generate() []byte {
	return basicValueMessage()
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