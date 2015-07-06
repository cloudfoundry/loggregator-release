package integration_test

import (
	"net"
	"time"

	"github.com/cloudfoundry/storeadapter"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Legacy message forwarding", func() {
	It("converts to events message format and forwards to doppler", func(done Done) {
		defer close(done)

		currentTime := time.Now()
		marshalledLegacyMessage := legacyLogMessage(123, "BLAH", currentTime)
		marshalledEventsEnvelope := addDefaultTags(eventsLogMessage(123, "BLAH", currentTime))
		marshalledEventsMessage, _ := proto.Marshal(marshalledEventsEnvelope)

		testServer := eventuallyListensForUDP("localhost:3457")
		defer testServer.Close()

		node := storeadapter.StoreNode{
			Key:   "/healthstatus/doppler/z1/0",
			Value: []byte("localhost"),
		}
		adapter := etcdRunner.Adapter()
		adapter.Create(node)

		connection, _ := net.Dial("udp", "localhost:51160")
		connection.Write(marshalledLegacyMessage)

		readBuffer := make([]byte, 65535)
		messageChan := make(chan []byte, 1000)

		go func() {
			for {
				readCount, _, _ := testServer.ReadFrom(readBuffer)
				readData := make([]byte, readCount)
				copy(readData, readBuffer[:readCount])

				if len(readData) > 32 {
					messageChan <- readData[32:]
				}
			}
		}()

		stopWrite := make(chan struct{})
		defer close(stopWrite)
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				connection.Write(marshalledLegacyMessage)

				select {
				case <-stopWrite:
					return
				case <-ticker.C:
				}
			}
		}()

		Eventually(messageChan).Should(Receive(Equal(marshalledEventsMessage)))
	}, 3)
})
