package integration_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"net"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/gomega"
)

func basicValueMetric(name string, value float64, unit string) *events.ValueMetric {
	return &events.ValueMetric{
		Name:  proto.String(name),
		Value: proto.Float64(value),
		Unit:  proto.String(unit),
	}
}

func addDefaultTags(envelope *events.Envelope) *events.Envelope {
	envelope.Deployment = proto.String("deployment-name")
	envelope.Job = proto.String("test-component")
	envelope.Index = proto.String("42")
	envelope.Ip = proto.String(localIPAddress)

	return envelope
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

func basicCounterEventMessage() []byte {
	message, _ := proto.Marshal(&events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_CounterEvent.Enum(),
		CounterEvent: &events.CounterEvent{
			Name:  proto.String("fake-counter-event-name"),
			Delta: proto.Uint64(1),
		},
	})

	return message
}

func basicHTTPStartMessage(requestId string) []byte {
	message, _ := proto.Marshal(basicHTTPStartMessageEnvelope(requestId))
	return message
}

func basicHTTPStartMessageEnvelope(requestId string) *events.Envelope {
	uuid, _ := uuid.ParseHex(requestId)
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_HttpStart.Enum(),
		HttpStart: &events.HttpStart{
			Timestamp:     proto.Int64(12),
			RequestId:     factories.NewUUID(uuid),
			PeerType:      events.PeerType_Client.Enum(),
			Method:        events.Method_GET.Enum(),
			Uri:           proto.String("some uri"),
			RemoteAddress: proto.String("some address"),
			UserAgent:     proto.String("some user agent"),
		},
	}
}

func basicHTTPStopMessage(requestId string) []byte {
	message, _ := proto.Marshal(basicHTTPStopMessageEnvelope(requestId))
	return message
}

func basicHTTPStopMessageEnvelope(requestId string) *events.Envelope {
	uuid, _ := uuid.ParseHex(requestId)
	return &events.Envelope{
		Origin:    proto.String("fake-origin-2"),
		EventType: events.Envelope_HttpStop.Enum(),
		HttpStop: &events.HttpStop{
			Timestamp:     proto.Int64(12),
			Uri:           proto.String("some uri"),
			RequestId:     factories.NewUUID(uuid),
			PeerType:      events.PeerType_Client.Enum(),
			StatusCode:    proto.Int32(404),
			ContentLength: proto.Int64(98475189),
		},
	}
}

func eventsLogMessage(appID int, message string, timestamp time.Time) *events.Envelope {
	return &events.Envelope{
		Origin:    proto.String("legacy"),
		EventType: events.Envelope_LogMessage.Enum(),
		LogMessage: &events.LogMessage{
			Message:        []byte(message),
			MessageType:    events.LogMessage_OUT.Enum(),
			Timestamp:      proto.Int64(timestamp.UnixNano()),
			AppId:          proto.String(string(appID)),
			SourceType:     proto.String("fake-source-id"),
			SourceInstance: proto.String("fake-source-id"),
		},
	}
}

func eventuallyListensForUDP(address string) net.PacketConn {
	var testServer net.PacketConn

	Eventually(func() error {
		var err error
		testServer, err = net.ListenPacket("udp4", address)
		return err
	}).Should(Succeed())

	return testServer
}

func eventuallyListensForTLS(address string) net.Listener {
	var listener net.Listener

	Eventually(func() error {
		var err error
		listener, err = net.Listen("tcp", address)
		return err
	}).Should(Succeed())

	return listener
}

type MetronInput struct {
	metronConn   net.Conn
	stopTheWorld chan struct{}
}

func (input *MetronInput) WriteToMetron(unsignedMessage []byte) {
	ticker := time.NewTicker(10 * time.Millisecond)

	for {
		select {
		case <-input.stopTheWorld:
			ticker.Stop()
			return
		case <-ticker.C:
			input.metronConn.Write(unsignedMessage)
		}
	}
}

type FakeDoppler struct {
	packetConn   net.PacketConn
	stopTheWorld chan struct{}
}

func (d *FakeDoppler) ReadIncomingMessages(signature []byte) chan signedMessage {
	messageChan := make(chan signedMessage, 1000)

	go func() {
		readBuffer := make([]byte, 65535)
		for {
			select {
			case <-d.stopTheWorld:
				return
			default:
				readCount, _, _ := d.packetConn.ReadFrom(readBuffer)
				readData := make([]byte, readCount)
				copy(readData, readBuffer[:readCount])

				// Only signed messages get placed on messageChan
				if gotSignedMessage(readData, signature) {
					msg := signedMessage{
						signature: readData[:len(signature)],
						message:   readData[len(signature):],
					}
					messageChan <- msg
				}
			}
		}
	}()

	return messageChan
}

func (d *FakeDoppler) Close() {
	d.packetConn.Close()
}

type signedMessage struct {
	signature []byte
	message   []byte
}

func sign(message []byte) signedMessage {
	expectedEnvelope := addDefaultTags(basicValueMessageEnvelope())
	expectedMessage, _ := proto.Marshal(expectedEnvelope)

	mac := hmac.New(sha256.New, []byte("shared_secret"))
	mac.Write(expectedMessage)

	signature := mac.Sum(nil)
	return signedMessage{signature: signature, message: expectedMessage}
}

func gotSignedMessage(readData, signature []byte) bool {
	return len(readData) > len(signature)
}
