package integration_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"net"
	"time"

	"github.com/cloudfoundry/storeadapter"
	"github.com/gogo/protobuf/proto"

	"bytes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dropsonde message forwarding", func() {
	var fakeDoppler *FakeDoppler
	var stopTheWorld chan struct{}

	BeforeEach(func() {
		node := storeadapter.StoreNode{
			Key:   "/healthstatus/doppler/z1/0",
			Value: []byte("localhost"),
		}

		adapter := etcdRunner.Adapter()
		adapter.Create(node)
		adapter.Disconnect()


		stopTheWorld = make(chan struct{})

		conn := eventuallyListensForUDP("localhost:3457")
		fakeDoppler = &FakeDoppler{
			packetConn: conn,
			stopTheWorld: stopTheWorld,
		}
	})

	AfterEach(func() {
		fakeDoppler.Close()
		close(stopTheWorld)
	})

	It("forwards hmac signed messages to a healthy doppler server", func(done Done) {
		defer close(done)

		originalMessage := basicValueMessage()
		expectedMessage := sign(originalMessage)

		receivedByDoppler := fakeDoppler.ReadIncomingMessages(expectedMessage.signature)

		metronConn, _ := net.Dial("udp4", "localhost:51161")
		metronInput := &MetronInput{
			metronConn: metronConn,
			stopTheWorld: stopTheWorld,
		}
		go metronInput.WriteToMetron(originalMessage)

		Eventually(func() bool {
			msg := <-receivedByDoppler
			return bytes.Equal(msg.signature, expectedMessage.signature) && bytes.Equal(msg.message, expectedMessage.message)
		}).Should(BeTrue())
	}, 2)
})

func sign(message []byte) signedMessage {
	expectedEnvelope := addDefaultTags(basicValueMessageEnvelope())
	expectedMessage, _ := proto.Marshal(expectedEnvelope)

	mac := hmac.New(sha256.New, []byte("shared_secret"))
	mac.Write(expectedMessage)

	signature := mac.Sum(nil)
	return signedMessage{ signature: signature, message: expectedMessage }
}

func gotSignedMessage(readData, signature []byte) bool {
	return len(readData) > len(signature)
}

type MetronInput struct {
	metronConn net.Conn
	stopTheWorld chan struct{}
}

func(input *MetronInput) WriteToMetron(unsignedMessage []byte) {
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
	packetConn net.PacketConn
	stopTheWorld chan struct{}
}

func(d *FakeDoppler) ReadIncomingMessages(signature []byte) chan signedMessage {
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
