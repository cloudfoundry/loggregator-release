package doppler_test

import (
	"net"
	"strconv"

	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/nu7hatch/gouuid"

	. "integration_tests/doppler/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Streaming Logs", func() {
	var inputConnection net.Conn
	var appID string

	BeforeEach(func() {
		guid, _ := uuid.NewV4()
		appID = guid.String()
	})

	itStreams := func(send func(event events.Event, connection net.Conn) error) {
		It("receives recent log messages", func() {
			logMessage := factories.NewLogMessage(events.LogMessage_OUT, "msg 1", appID, "APP")
			err := send(logMessage, inputConnection)
			Expect(err).NotTo(HaveOccurred())

			returnedMessages := make([][]byte, 1)
			Eventually(func() [][]byte {
				returnedMessages = retreiveRecentMessages(appID)
				return returnedMessages
			}).Should(HaveLen(1))

			receivedMessage := DecodeProtoBufLogMessage(returnedMessages[0])

			Expect(receivedMessage.GetAppId()).To(Equal(appID))
			Expect(string(receivedMessage.GetMessage())).To(Equal("msg 1"))
		})

		It("only recieves messages for the specified appId", func() {
			logMessage := factories.NewLogMessage(events.LogMessage_OUT, "msg 1", appID, "APP")
			err := send(logMessage, inputConnection)
			Expect(err).NotTo(HaveOccurred())

			logMessage = factories.NewLogMessage(events.LogMessage_OUT, "msg 2", "otherId", "APP")
			err = send(logMessage, inputConnection)
			Expect(err).NotTo(HaveOccurred())

			returnedMessages := make([][]byte, 1)
			Eventually(func() [][]byte {
				returnedMessages = retreiveRecentMessages(appID)
				return returnedMessages
			}).Should(HaveLen(1))

			receivedMessage := DecodeProtoBufLogMessage(returnedMessages[0])

			Expect(receivedMessage.GetAppId()).To(Equal(appID))
			Expect(string(receivedMessage.GetMessage())).To(Equal("msg 1"))
		})

		It("does not recieve non-log messages", func() {
			metricEvent := factories.NewContainerMetric(appID, 0, 10, 10, 10)
			err := send(metricEvent, inputConnection)
			Expect(err).NotTo(HaveOccurred())
			returnedMessages := make([][]byte, 1)

			Consistently(func() [][]byte {
				returnedMessages = retreiveRecentMessages(appID)
				return returnedMessages
			}).Should(HaveLen(0))
		})

		It("only recieves the most recent logs", func() {
			for i := 0; i < 15; i++ {
				logMessage := factories.NewLogMessage(events.LogMessage_OUT, strconv.Itoa(i), appID, "APP")
				err := send(logMessage, inputConnection)
				Expect(err).NotTo(HaveOccurred())
			}

			Eventually(func() []string {
				returnedMessages := retreiveRecentMessages(appID)
				var messages []string

				for _, messageBytes := range returnedMessages {
					receivedMessage := DecodeProtoBufLogMessage(messageBytes)
					messages = append(messages, string(receivedMessage.GetMessage()))
				}

				return messages
			}).Should(Equal([]string{"5", "6", "7", "8", "9", "10", "11", "12", "13", "14"}))
		})
	}

	Context("TLS", func() {
		BeforeEach(func() {
			var err error
			inputConnection, err = DialTLS(localIPAddress+":8766", "../fixtures/client.crt", "../fixtures/client.key", "../fixtures/loggregator-ca.crt")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			inputConnection.Close()
		})

		itStreams(SendEventTCP)
	})

	Context("UDP", func() {
		BeforeEach(func() {
			inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
		})

		AfterEach(func() {
			inputConnection.Close()
		})

		itStreams(SendEvent)
	})
})

func retreiveRecentMessages(appID string) [][]byte {
	rChan := make(chan []byte, 10)

	ws, _ := AddWSSink(rChan, "4567", "/apps/"+appID+"/recentlogs")
	defer ws.Close()

	returnedMessages := make([][]byte, 0)
	for message := range rChan {
		returnedMessages = append(returnedMessages, message)
	}

	return returnedMessages
}
