package doppler_test

import (
	"net"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Streaming Logs", func() {
	var ws *websocket.Conn
	var connDropped <-chan struct{}
	var receivedChan chan []byte
	var inputConnection net.Conn

	BeforeEach(func() {
		receivedChan = make(chan []byte, 10)
		ws, connDropped = addWSSink(receivedChan, "4567", "/apps/appId/stream")
		inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
	})

	AfterEach(func() {
		ws.Close()
		Eventually(connDropped).Should(BeClosed())
	})

	It("streams logs for an app", func() {
		err := sendAppLog("appId", "message", inputConnection)
		Expect(err).NotTo(HaveOccurred())

		receivedMessageBytes := []byte{}
		Eventually(receivedChan).Should(Receive(&receivedMessageBytes))
		receivedMessage := decodeProtoBufLogMessage(receivedMessageBytes)

		Expect(receivedMessage.GetAppId()).To(Equal("appId"))
		Expect(string(receivedMessage.GetMessage())).To(Equal("message"))

	})

	It("only recieves messages for the specified appId", func() {
		err := sendAppLog("appId", "message 1", inputConnection)
		Expect(err).NotTo(HaveOccurred())

		err = sendAppLog("otherAppId", "message 2", inputConnection)
		Expect(err).NotTo(HaveOccurred())

		receivedMessageBytes := []byte{}
		Eventually(receivedChan).Should(Receive(&receivedMessageBytes))
		receivedMessage := decodeProtoBufLogMessage(receivedMessageBytes)

		Expect(receivedMessage.GetAppId()).To(Equal("appId"))
		Expect(string(receivedMessage.GetMessage())).To(Equal("message 1"))
		Expect(receivedChan).To(BeEmpty())
	})

	It("does not recieve non-log messages", func() {
		metricEvent := factories.NewContainerMetric("appId", 0, 10, 10, 10)
		sendEvent(metricEvent, inputConnection)

		Expect(receivedChan).To(BeEmpty())
	})
})
