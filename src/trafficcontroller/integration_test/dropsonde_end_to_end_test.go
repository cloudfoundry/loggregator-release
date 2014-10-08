package integration_test

import (
	"crypto/tls"
	"fmt"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/noaa"
	"net/http"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var dropsondeEndpoint string

const DOPPLER_DROPSONDE_PORT = 4566

var _ = Describe("TrafficController for dropsonde messages", func() {
	BeforeEach(func() {
		dropsondeEndpoint = fmt.Sprintf("ws://%s:%d", localIPAddress, DOPPLER_DROPSONDE_PORT)
		fakeDoppler.ResetMessageChan()
	})

	Context("Streaming", func() {
		It("passes messages through", func() {
			client := noaa.NewNoaa(dropsondeEndpoint, &tls.Config{}, nil)
			messages, err := client.Stream(APP_ID, AUTH_TOKEN)
			Expect(err).NotTo(HaveOccurred())

			var request *http.Request
			Eventually(fakeDoppler.TrafficControllerConnected, 10).Should(Receive(&request))
			Expect(request.URL.Path).To(Equal("/apps/1234/stream"))

			currentTime := time.Now().UnixNano()
			dropsondeMessage := makeDropsondeMessage("Hello through NOAA", APP_ID, currentTime)
			fakeDoppler.SendLogMessage(dropsondeMessage)

			var receivedEnvelope *events.Envelope
			Eventually(messages).Should(Receive(&receivedEnvelope))

			receivedMessage := receivedEnvelope.GetLogMessage()
			Expect(receivedMessage.GetMessage()).To(BeEquivalentTo("Hello through NOAA"))
			Expect(receivedMessage.GetAppId()).To(Equal(APP_ID))
			Expect(receivedMessage.GetTimestamp()).To(Equal(currentTime))

			client.Close()
		})
	})

	Context("Firehose", func() {
		It("passes messages through for every app for uaa admins", func() {
			client := noaa.NewNoaa(dropsondeEndpoint, &tls.Config{}, nil)
			messages, err := client.Firehose(AUTH_TOKEN)
			Expect(err).NotTo(HaveOccurred())

			var request *http.Request
			Eventually(fakeDoppler.TrafficControllerConnected, 10).Should(Receive(&request))
			Expect(request.URL.Path).To(Equal("/firehose"))

			currentTime := time.Now().UnixNano()
			dropsondeMessage := makeDropsondeMessage("Hello through NOAA", APP_ID, currentTime)
			fakeDoppler.SendLogMessage(dropsondeMessage)

			var receivedEnvelope *events.Envelope
			Eventually(messages).Should(Receive(&receivedEnvelope))

			receivedMessage := receivedEnvelope.GetLogMessage()
			Expect(receivedMessage.GetMessage()).To(BeEquivalentTo("Hello through NOAA"))
			Expect(receivedMessage.GetAppId()).To(Equal(APP_ID))
			Expect(receivedMessage.GetTimestamp()).To(Equal(currentTime))

			client.Close()
		})
	})

	Context("Recent", func() {
		var expectedMessages [][]byte

		BeforeEach(func() {
			expectedMessages = make([][]byte, 5)

			for i := 0; i < 5; i++ {
				message := makeDropsondeMessage(strconv.Itoa(i), "1234", 1234)
				expectedMessages[i] = message
				fakeDoppler.SendLogMessage(message)
			}
			fakeDoppler.CloseLogMessageStream()
		})

		It("returns a multi-part HTTP response with all recent messages", func(done Done) {
			client := noaa.NewNoaa(dropsondeEndpoint, &tls.Config{}, nil)

			messages, err := client.RecentLogs("1234", "bearer iAmAnAdmin")

			var request *http.Request
			Eventually(fakeDoppler.TrafficControllerConnected, 15).Should(Receive(&request))
			Expect(request.URL.Path).To(Equal("/apps/1234/recentlogs"))

			Expect(err).NotTo(HaveOccurred())

			for i, message := range messages {
				Expect(message.GetLogMessage().GetMessage()).To(BeEquivalentTo(strconv.Itoa(i)))
			}
			close(done)
		}, 20)
	})
})
