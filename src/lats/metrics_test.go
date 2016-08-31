package lats_test

import (
	"lats/helpers"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	counterEvent = &events.CounterEvent{
		Name:  proto.String("LATs-Counter"),
		Delta: proto.Uint64(5),
		Total: proto.Uint64(5),
	}
	valueMetric = &events.ValueMetric{
		Name:  proto.String("LATs-Value"),
		Value: proto.Float64(10),
		Unit:  proto.String("test-unit"),
	}
	containerMetric = &events.ContainerMetric{
		ApplicationId: proto.String("id"),
		InstanceIndex: proto.Int32(1),
		CpuPercentage: proto.Float64(20.0),
		MemoryBytes:   proto.Uint64(10),
		DiskBytes:     proto.Uint64(20),
	}
)

var _ = Describe("Sending metrics through loggregator", func() {
	var (
		msgChan   chan *events.Envelope
		errorChan chan error
	)
	BeforeEach(func() {
		msgChan, errorChan = helpers.ConnectToFirehose()
	})

	AfterEach(func() {
		Expect(errorChan).To(BeEmpty())
	})

	Context("When a counter event is emitted to metron", func() {
		It("Gets through the firehose", func() {
			envelope := createCounterEvent()
			helpers.EmitToMetron(envelope)

			receivedEnvelope := helpers.FindMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())

			receivedCounterEvent := receivedEnvelope.GetCounterEvent()
			Expect(receivedCounterEvent).To(Equal(counterEvent))
			helpers.EmitToMetron(envelope)

			receivedEnvelope = helpers.FindMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())

			receivedCounterEvent = receivedEnvelope.GetCounterEvent()
			Expect(receivedCounterEvent.GetTotal()).To(Equal(uint64(10)))
		})
	})

	Context("When a value metric is emitted to metron", func() {
		It("Gets through the firehose", func() {
			envelope := createValueMetric()
			helpers.EmitToMetron(envelope)

			receivedEnvelope := helpers.FindMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())

			receivedValueMetric := receivedEnvelope.GetValueMetric()
			Expect(receivedValueMetric).To(Equal(valueMetric))
		})
	})

	Context("When a container metric is emitted to metron", func() {
		It("Gets through the firehose", func() {
			envelope := createContainerMetric()
			helpers.EmitToMetron(envelope)

			receivedEnvelope := helpers.FindMatchingEnvelope(msgChan)
			Expect(receivedEnvelope).NotTo(BeNil())

			receivedContainerMetric := receivedEnvelope.GetContainerMetric()
			Expect(receivedContainerMetric).To(Equal(containerMetric))
		})
	})
})

func createCounterEvent() *events.Envelope {
	return &events.Envelope{
		Origin:       proto.String(helpers.ORIGIN_NAME),
		EventType:    events.Envelope_CounterEvent.Enum(),
		Timestamp:    proto.Int64(time.Now().UnixNano()),
		CounterEvent: counterEvent,
	}
}

func createValueMetric() *events.Envelope {
	return &events.Envelope{
		Origin:      proto.String(helpers.ORIGIN_NAME),
		EventType:   events.Envelope_ValueMetric.Enum(),
		Timestamp:   proto.Int64(time.Now().UnixNano()),
		ValueMetric: valueMetric,
	}
}

func createContainerMetric() *events.Envelope {
	return &events.Envelope{
		Origin:          proto.String(helpers.ORIGIN_NAME),
		EventType:       events.Envelope_ContainerMetric.Enum(),
		Timestamp:       proto.Int64(time.Now().UnixNano()),
		ContainerMetric: containerMetric,
	}
}
