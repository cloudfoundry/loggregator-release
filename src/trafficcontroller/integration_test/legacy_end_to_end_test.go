package integration_test

import (
	"crypto/tls"
	"fmt"
	"github.com/cloudfoundry/loggregator_consumer"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"net/http"
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

			stopChan := make(chan struct{})
			receivedMessages := []*logmessage.LogMessage{}

			go func() {
				for {
					select {
					case <-stopChan:
						return
					case message, ok := <-messages:
						if !ok {
							return
						}
						receivedMessages = append(receivedMessages, message)
					}
				}
			}()

			var request *http.Request
			Eventually(fakeDoppler.TrafficControllerConnected, 10).Should(Receive(&request))
			Expect(request.URL.Path).To(Equal("/apps/1234/stream"))

			currentTime := time.Now().UnixNano()
			dropsondeMessage := makeDropsondeMessage("Make me Legacy Format", APP_ID, currentTime)
			fakeDoppler.SendLogMessage(dropsondeMessage)

			Eventually(func() []*logmessage.LogMessage { return receivedMessages }).Should(HaveLen(1))
			receivedMessage := receivedMessages[0]
			Expect(receivedMessage.GetMessage()).To(BeEquivalentTo("Make me Legacy Format"))
			Expect(receivedMessage.GetAppId()).To(Equal(APP_ID))
			Expect(receivedMessage.GetTimestamp()).To(Equal(currentTime))
			close(stopChan)
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
})
