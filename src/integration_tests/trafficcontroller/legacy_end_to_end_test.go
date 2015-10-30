package integration_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cloudfoundry/loggregator_consumer"
	"github.com/cloudfoundry/loggregatorlib/logmessage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"integration_tests/trafficcontroller/fake_doppler"
)

var legacyEndpoint string

const TRAFFIC_CONTROLLER_LEGACY_PORT = 4567

var _ = Describe("TrafficController for legacy messages", func() {
	BeforeEach(func() {
		fakeDoppler = fake_doppler.New()
		go fakeDoppler.Start()
		legacyEndpoint = fmt.Sprintf("ws://%s:%d", localIPAddress, TRAFFIC_CONTROLLER_LEGACY_PORT)

		Eventually(func() error {
			_, err := http.Get(fmt.Sprintf("http://%s:%d", localIPAddress, TRAFFIC_CONTROLLER_LEGACY_PORT))
			return err
		}).ShouldNot(HaveOccurred())

	})

	AfterEach(func() {
		fakeDoppler.Stop()
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
		It("returns a multi-part HTTP response with all recent messages", func() {
			const messageLength = 5
			expectedMessages := make([][]byte, messageLength)

			for i := 0; i < messageLength; i++ {
				message := makeDropsondeMessage(strconv.Itoa(i), "1234", 1234)
				expectedMessages[i] = message
				fakeDoppler.SendLogMessage(message)
			}

			fakeDoppler.CloseLogMessageStream()
			Eventually(func() bool {
				client := loggregator_consumer.New(legacyEndpoint, &tls.Config{}, nil)

				messages, err := client.Recent(APP_ID, AUTH_TOKEN)
				Expect(err).NotTo(HaveOccurred())

				select {
				case request := <-fakeDoppler.TrafficControllerConnected:
					Expect(request.URL.Path).To(Equal("/apps/1234/recentlogs"))

					Expect(messages).To(HaveLen(messageLength))
					for i, message := range messages {
						Expect(message.GetMessage()).To(BeEquivalentTo(strconv.Itoa(i)))
					}
					return true
				default:
					return false
				}
			}, 5).Should(BeTrue())
		})

		It("correctly handles when clients go away mid-stream", func() {
			recentPath := fmt.Sprintf("http://%s:%d/recent?app=1234", localIPAddress, TRAFFIC_CONTROLLER_LEGACY_PORT)
			client := &http.Client{}

			req, _ := http.NewRequest("GET", recentPath, nil)
			req.Header.Set("Authorization", "iAmNotAnAdmin")

			// write many messages to make sure the http handler flushes the
			// response headers which allow client.Do() to return
			message := makeDropsondeMessage("foo", "1234", 1234)
			for i := 0; i < 50; i++ {
				fakeDoppler.SendLogMessage(message)
			}

			stopConnecting := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				for {
					resp, err := client.Do(req)
					Expect(err).NotTo(HaveOccurred())

					resp.Body.Close()
					select {
					case <-stopConnecting:
						return
					case <-time.After(100 * time.Millisecond):
					}
				}
			}()

			var request *http.Request
			Eventually(fakeDoppler.TrafficControllerConnected, 5).Should(Receive(&request))
			close(stopConnecting)

			// write many messages to make sure we flush the connection and
			// cause an error in the http handler
			for i := 0; i < 100; i++ {
				message := makeDropsondeMessage("foo", "1234", 1234)
				fakeDoppler.SendLogMessage(message)
			}
			fakeDoppler.CloseLogMessageStream()

			Consistently(trafficControllerSession.Err.Contents).ShouldNot(ContainSubstring("panic serving"))
		})
	})

	Context("SetCookie", func() {
		It("sets the desired cookie on the response", func() {
			response, err := http.PostForm(fmt.Sprintf("http://%s:%d/set-cookie", localIPAddress, TRAFFIC_CONTROLLER_LEGACY_PORT), url.Values{"CookieName": {"authorization"}, "CookieValue": {url.QueryEscape("bearer iAmAnAdmin")}})
			Expect(err).NotTo(HaveOccurred())

			Expect(response.Cookies()).NotTo(BeNil())
			Expect(response.Cookies()).To(HaveLen(1))
			cookie := response.Cookies()[0]
			Expect(cookie.Domain).To(Equal("loggregator.vcap.me"))
			Expect(cookie.Name).To(Equal("authorization"))
			Expect(cookie.Value).To(Equal("bearer+iAmAnAdmin"))
			Expect(cookie.Secure).To(BeTrue())
		})
	})

})
