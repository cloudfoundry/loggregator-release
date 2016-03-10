package statsdemitter_test

import (
	"net"
	"statsd-injector/statsdemitter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("Statsdemitter", func() {
	var (
		udpListener *net.UDPConn
	)

	var _ = BeforeEach(func() {
		udpAddr, _ := net.ResolveUDPAddr("udp", ":8088")
		udpListener, _ = net.ListenUDP("udp", udpAddr)
		loggertesthelper.TestLoggerSink.Clear()
	})

	var _ = AfterEach(func() {
		udpListener.Close()
	})

	It("emits the serialized envelope on the given UDP port", func(done Done) {
		defer close(done)
		inputChan := make(chan *events.Envelope)
		emitter := statsdemitter.New(8088, loggertesthelper.Logger())
		go emitter.Run(inputChan)
		message := &events.Envelope{
			Origin:    proto.String("origin"),
			Timestamp: proto.Int64(3000000000),
			EventType: events.Envelope_CounterEvent.Enum(),
			CounterEvent: &events.CounterEvent{
				Name:  proto.String("counterName"),
				Delta: proto.Uint64(3),
				Total: proto.Uint64(15),
			},
		}

		inputChan <- message

		buffer := make([]byte, 4096)
		readCount, _, err := udpListener.ReadFromUDP(buffer)
		Expect(err).ToNot(HaveOccurred())

		received := buffer[:readCount]

		expectedBytes, err := proto.Marshal(message)
		Expect(err).NotTo(HaveOccurred())
		Expect(received).To(Equal(expectedBytes))
	})

	It("does not emit invalid envelope", func(done Done) {
		defer close(done)
		inputChan := make(chan *events.Envelope)
		emitter := statsdemitter.New(8088, loggertesthelper.Logger())
		go emitter.Run(inputChan)

		badMessage := &events.Envelope{
			Timestamp: proto.Int64(3000000000),
			EventType: events.Envelope_CounterEvent.Enum(),
		}
		goodMessage := &events.Envelope{
			Origin:    proto.String("origin"),
			Timestamp: proto.Int64(3000000000),
			EventType: events.Envelope_CounterEvent.Enum(),
			CounterEvent: &events.CounterEvent{
				Name:  proto.String("counterName"),
				Delta: proto.Uint64(3),
				Total: proto.Uint64(15),
			},
		}

		inputChan <- badMessage
		inputChan <- goodMessage

		buffer := make([]byte, 4096)
		readCount, _, _ := udpListener.ReadFromUDP(buffer)

		received := buffer[:readCount]

		expectedBytes, err := proto.Marshal(goodMessage)
		Expect(err).NotTo(HaveOccurred())
		Expect(received).To(Equal(expectedBytes))
	})

})
