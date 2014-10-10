package integration_test

import (
	"crypto/tls"
	"fmt"
	"github.com/cloudfoundry/loggregator_consumer"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"net/http"
	"net/url"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var legacyEndpoint string

const DOPPLER_LEGACY_PORT = 4567

var _ = Describe("TrafficController for legacy messages", func() {
	BeforeEach(func() {
		legacyEndpoint = fmt.Sprintf("ws://%s:%d", localIPAddress, DOPPLER_LEGACY_PORT)
		fakeDoppler.ResetMessageChan()
	})

	Context("Streaming", func() {
		It("delivers legacy format messages at legacy endpoint", func() {
			legacy_consumer := loggregator_consumer.New(legacyEndpoint, &tls.Config{}, nil)
			messages, err := legacy_consumer.Tail(APP_ID, AUTH_TOKEN)
			Expect(err).NotTo(HaveOccurred())

			var request *http.Request
			Eventually(fakeDoppler.TrafficControllerConnected, 10).Should(Receive(&request))
			Expect(request.URL.Path).To(Equal("/apps/1234/stream"))

			currentTime := time.Now().UnixNano()
			dropsondeMessage := makeDropsondeMessage("Make me Legacy Format", APP_ID, currentTime)
			fakeDoppler.SendLogMessage(dropsondeMessage)

			var receivedMessage *logmessage.LogMessage
			Eventually(messages).Should(Receive(&receivedMessage))

			Expect(receivedMessage.GetMessage()).To(BeEquivalentTo("Make me Legacy Format"))
			Expect(receivedMessage.GetAppId()).To(Equal(APP_ID))
			Expect(receivedMessage.GetTimestamp()).To(Equal(currentTime))

			legacy_consumer.Close()
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
			client := loggregator_consumer.New(legacyEndpoint, &tls.Config{}, nil)

			messages, err := client.Recent("1234", "bearer iAmAnAdmin")

			var request *http.Request
			Eventually(fakeDoppler.TrafficControllerConnected, 15).Should(Receive(&request))
			Expect(request.URL.Path).To(Equal("/apps/1234/recentlogs"))

			Expect(err).NotTo(HaveOccurred())

			for i, message := range messages {
				Expect(message.GetMessage()).To(BeEquivalentTo(strconv.Itoa(i)))
			}
			close(done)
		}, 20)
	})

	Context("SetCookie", func() {
		It("sets the desired cookie on the response", func() {
			response, err := http.PostForm(fmt.Sprintf("http://%s:%d/set-cookie", localIPAddress, DOPPLER_LEGACY_PORT), url.Values{"CookieName": {"authorization"}, "CookieValue": {url.QueryEscape("bearer iAmAnAdmin")}})
			Expect(err).NotTo(HaveOccurred())

			Expect(response.Cookies()).NotTo(BeNil())
			Expect(response.Cookies()).To(HaveLen(1))
			cookie := response.Cookies()[0]
			Expect(cookie.Domain).To(Equal("loggregator.vcap.me"))
			Expect(cookie.Name).To(Equal("authorization"))
			Expect(cookie.Value).To(Equal("bearer iAmAnAdmin"))
		})
	})
})
