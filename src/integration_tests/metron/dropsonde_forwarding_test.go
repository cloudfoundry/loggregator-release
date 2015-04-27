package integration_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"net"
	"time"

	"github.com/cloudfoundry/storeadapter"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/gogo/protobuf/proto"
)

var _ = Describe("Dropsonde message forwarding", func() {
	var testDoppler net.PacketConn

	BeforeEach(func() {
		testDoppler, _ = net.ListenPacket("udp", "localhost:3457")

		node := storeadapter.StoreNode{
			Key:   "/healthstatus/doppler/z1/0",
			Value: []byte("localhost"),
		}

		adapter := etcdRunner.Adapter()
		adapter.Create(node)
		adapter.Disconnect()
	})

	AfterEach(func() {
		testDoppler.Close()
	})

	It("forwards hmac signed messages to a healthy doppler server", func(done Done) {
		type signedMessage struct {
			signature []byte
			message   []byte
		}

		defer close(done)

		originalMessage := basicHeartbeatMessage()
		expectedEnvelope := basicHeartbeatEvent()
		expectedMessage, _ := proto.Marshal(expectedEnvelope)

		mac := hmac.New(sha256.New, []byte("shared_secret"))
		mac.Write(expectedMessage)

		signature := mac.Sum(nil)

		metronInput, _ := net.Dial("udp", "localhost:51161")

		messageChan := make(chan signedMessage, 1000)

		stopTheWorld := make(chan struct{})
		defer close(stopTheWorld)

		readFromDoppler := func() {
			gotSignedMessage := func(readData []byte) bool {
				return len(readData) > len(signature)
			}

			readBuffer := make([]byte, 65535)

			for {
				readCount, _, _ := testDoppler.ReadFrom(readBuffer)
				readData := make([]byte, readCount)
				copy(readData, readBuffer[:readCount])

				if gotSignedMessage(readData) {
					messageChan <- signedMessage{signature: readData[:len(signature)], message: readData[len(signature):]}
				}

				select {
				case <-stopTheWorld:
					return
				default:
				}
			}
		}

		go readFromDoppler()

		writeToMetron := func() {
			ticker := time.NewTicker(10 * time.Millisecond)

			for {
				metronInput.Write(originalMessage)

				select {
				case <-stopTheWorld:
					ticker.Stop()
					return
				case <-ticker.C:
				}
			}
		}

		go writeToMetron()

		var receivedMessage signedMessage
		Eventually(messageChan).Should(Receive(&receivedMessage))

		Expect(receivedMessage).To(Equal(signedMessage{signature: signature, message: expectedMessage}))

	})
})
