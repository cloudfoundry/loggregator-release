package doppler_test

import (
	"net"
	"strconv"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/nu7hatch/gouuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Streaming Logs", func() {
	var inputConnection net.Conn
	var appID string

	BeforeEach(func() {
		guid, _ := uuid.NewV4()
		appID = guid.String()

		inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
	})

	It("receives recent log messages", func() {
		err := sendAppLog(appID, "message 1", inputConnection)
		Expect(err).NotTo(HaveOccurred())

		returnedMessages := make([][]byte, 1)
		Eventually(func() [][]byte {
			returnedMessages = retreiveRecentMessages(appID)
			return returnedMessages
		}).Should(HaveLen(1))

		receivedMessage := decodeProtoBufLogMessage(returnedMessages[0])

		Expect(receivedMessage.GetAppId()).To(Equal(appID))
		Expect(string(receivedMessage.GetMessage())).To(Equal("message 1"))
	})

	It("only recieves messages for the specified appId", func() {
		err := sendAppLog(appID, "message 1", inputConnection)
		Expect(err).NotTo(HaveOccurred())

		err = sendAppLog("otherAppId", "message 2", inputConnection)
		Expect(err).NotTo(HaveOccurred())

		returnedMessages := make([][]byte, 1)
		Eventually(func() [][]byte {
			returnedMessages = retreiveRecentMessages(appID)
			return returnedMessages
		}).Should(HaveLen(1))

		receivedMessage := decodeProtoBufLogMessage(returnedMessages[0])

		Expect(receivedMessage.GetAppId()).To(Equal(appID))
		Expect(string(receivedMessage.GetMessage())).To(Equal("message 1"))
	})

	It("does not recieve non-log messages", func() {
		metricEvent := factories.NewContainerMetric(appID, 0, 10, 10, 10)
		sendEvent(metricEvent, inputConnection)
		returnedMessages := make([][]byte, 1)

		Consistently(func() [][]byte {
			returnedMessages = retreiveRecentMessages(appID)
			return returnedMessages
		}).Should(HaveLen(0))
	})

	It("only recieves the most recent logs", func() {
		for i := 0; i < 15; i++ {
			err := sendAppLog(appID, strconv.Itoa(i), inputConnection)
			Expect(err).NotTo(HaveOccurred())
		}

		Eventually(func() []string {
			returnedMessages := retreiveRecentMessages(appID)
			var messages []string

			for _, messageBytes := range returnedMessages {
				receivedMessage := decodeProtoBufLogMessage(messageBytes)
				messages = append(messages, string(receivedMessage.GetMessage()))
			}

			return messages
		}).Should(Equal([]string{"5", "6", "7", "8", "9", "10", "11", "12", "13", "14"}))
	})
})

func retreiveRecentMessages(appID string) [][]byte {
	rChan := make(chan []byte, 10)

	ws, _ := addWSSink(rChan, "4567", "/apps/"+appID+"/recentlogs")
	defer ws.Close()

	returnedMessages := make([][]byte, 0)
	for message := range rChan {
		returnedMessages = append(returnedMessages, message)
	}

	return returnedMessages
}
