package doppler_test

import (
	"net"
	"time"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gorilla/websocket"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// WebsocketSinks are a deprecated code path
var _ = XDescribe("Websocket Streaming Logs", func() {
	var ws *websocket.Conn
	var connDropped <-chan struct{}
	var receivedChan chan []byte
	var inputConnection net.Conn
	var appID string

	BeforeEach(func() {
		receivedChan = make(chan []byte, 10)

		guid, _ := uuid.NewV4()
		appID = guid.String()

		ws, connDropped = AddWSSink(receivedChan, "4567", "/apps/"+appID+"/stream")
		inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
		time.Sleep(50 * time.Millisecond) // give time for connection to establish
	})

	AfterEach(func() {
		ws.Close()
		Eventually(connDropped).Should(BeClosed())
	})

	It("streams logs for an app", func() {
		err := SendAppLog(appID, "message", inputConnection)
		Expect(err).NotTo(HaveOccurred())

		receivedMessageBytes := []byte{}
		Eventually(receivedChan).Should(Receive(&receivedMessageBytes))
		receivedMessage := DecodeProtoBufLogMessage(receivedMessageBytes)

		Expect(receivedMessage.GetAppId()).To(Equal(appID))
		Expect(string(receivedMessage.GetMessage())).To(Equal("message"))

	})

	It("only receives messages for the specified appId", func() {
		err := SendAppLog(appID, "message 1", inputConnection)
		Expect(err).NotTo(HaveOccurred())

		err = SendAppLog("otherAppId", "message 2", inputConnection)
		Expect(err).NotTo(HaveOccurred())

		receivedMessageBytes := []byte{}
		Eventually(receivedChan).Should(Receive(&receivedMessageBytes))
		receivedMessage := DecodeProtoBufLogMessage(receivedMessageBytes)

		Expect(receivedMessage.GetAppId()).To(Equal(appID))
		Expect(string(receivedMessage.GetMessage())).To(Equal("message 1"))
		Expect(receivedChan).To(BeEmpty())
	})

	It("does not receive non-log messages", func() {
		metricEvent := factories.NewContainerMetric(appID, 0, 10, 10, 10)
		SendEvent(metricEvent, inputConnection)

		Expect(receivedChan).To(BeEmpty())
	})

	It("drops invalid log envelopes", func() {
		unmarshalledLogMessage := factories.NewLogMessage(events.LogMessage_OUT, "Some Data", appID, "App")
		expectedMessage := MarshalEvent(unmarshalledLogMessage, "invalid")

		_, err := inputConnection.Write(expectedMessage)
		Expect(err).To(BeNil())
		Expect(receivedChan).To(BeEmpty())
	})
})
