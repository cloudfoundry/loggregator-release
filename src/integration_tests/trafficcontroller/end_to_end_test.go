package trafficcontroller_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("TrafficController for v1 messages", func() {
	var (
		logCache     *stubGrpcLogCache
		fakeDoppler  *FakeDoppler
		tcWSEndpoint string
		wsPort       int
		httpClient   = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	)

	Context("without a configured log cache", func() {
		BeforeEach(func() {
			fakeDoppler = NewFakeDoppler()
			err := fakeDoppler.Start()
			Expect(err).ToNot(HaveOccurred())

			cfg := testservers.BuildTrafficControllerConfWithoutLogCache(fakeDoppler.Addr(), 37474)

			var tcPorts testservers.TrafficControllerPorts
			tcCleanupFunc, tcPorts = testservers.StartTrafficController(cfg)

			wsPort = tcPorts.WS

			tcHTTPEndpoint := fmt.Sprintf("https://%s:%d", localIPAddress, tcPorts.WS)
			tcWSEndpoint = fmt.Sprintf("wss://%s:%d", localIPAddress, tcPorts.WS)

			// wait for TC
			Eventually(func() error {
				resp, err := httpClient.Get(tcHTTPEndpoint)
				if err == nil {
					resp.Body.Close()
				}
				return err
			}, 10).Should(Succeed())
		})

		AfterEach(func(done Done) {
			defer close(done)
			fakeDoppler.Stop()
		}, 30)

		Describe("LogCache API Paths", func() {
			Context("Recent", func() {
				It("returns a helpful error message", func() {
					client := consumer.New(tcWSEndpoint, &tls.Config{
						InsecureSkipVerify: true,
					}, nil)

					logMessages, err := client.RecentLogs("efe5c422-e8a7-42c2-a52b-98bffd8d6a07", "bearer iAmAnAdmin")

					Expect(err).ToNot(HaveOccurred())
					Expect(logMessages).To(HaveLen(1))
					Expect(string(logMessages[0].GetMessage())).To(Equal("recent log endpoint requires a log cache. please talk to you operator"))
				})
			})
		})
	})

	Context("with a configured log cache", func() {
		BeforeEach(func() {
			fakeDoppler = NewFakeDoppler()
			err := fakeDoppler.Start()
			Expect(err).ToNot(HaveOccurred())

			logCache = newStubGrpcLogCache()
			cfg := testservers.BuildTrafficControllerConf(fakeDoppler.Addr(), 37474, logCache.addr())

			var tcPorts testservers.TrafficControllerPorts
			tcCleanupFunc, tcPorts = testservers.StartTrafficController(cfg)

			wsPort = tcPorts.WS

			tcHTTPEndpoint := fmt.Sprintf("https://%s:%d", localIPAddress, tcPorts.WS)
			tcWSEndpoint = fmt.Sprintf("wss://%s:%d", localIPAddress, tcPorts.WS)

			// wait for TC
			Eventually(func() error {
				resp, err := httpClient.Get(tcHTTPEndpoint)
				if err == nil {
					resp.Body.Close()
				}
				return err
			}, 10).Should(Succeed())
		})

		AfterEach(func(done Done) {
			defer close(done)
			fakeDoppler.Stop()
			logCache.stop()
		}, 30)

		Describe("Loggregator Router Paths", func() {
			Context("Streaming", func() {
				var (
					client   *consumer.Consumer
					messages <-chan *events.Envelope
					errors   <-chan error
				)

				BeforeEach(func() {
					client = consumer.New(tcWSEndpoint, &tls.Config{
						InsecureSkipVerify: true,
					}, nil)
					messages, errors = client.StreamWithoutReconnect(APP_ID, AUTH_TOKEN)
				})

				It("passes messages through", func() {
					var grpcRequest *plumbing.SubscriptionRequest
					Eventually(fakeDoppler.SubscriptionRequests, 10).Should(Receive(&grpcRequest))
					Expect(grpcRequest.Filter).ToNot(BeNil())
					Expect(grpcRequest.Filter.AppID).To(Equal(APP_ID))

					currentTime := time.Now().UnixNano()
					dropsondeMessage := makeDropsondeMessage("Hello through NOAA", APP_ID, currentTime)
					fakeDoppler.SendLogMessage(dropsondeMessage)

					var receivedEnvelope *events.Envelope
					Eventually(messages).Should(Receive(&receivedEnvelope))
					Consistently(errors).ShouldNot(Receive())

					receivedMessage := receivedEnvelope.GetLogMessage()
					Expect(receivedMessage.GetMessage()).To(BeEquivalentTo("Hello through NOAA"))
					Expect(receivedMessage.GetAppId()).To(Equal(APP_ID))
					Expect(receivedMessage.GetTimestamp()).To(Equal(currentTime))

					client.Close()
				})

				It("closes the upstream websocket connection when done", func() {
					var server plumbing.Doppler_BatchSubscribeServer
					Eventually(fakeDoppler.SubscribeServers, 10).Should(Receive(&server))

					client.Close()

					Eventually(server.Context().Done()).Should(BeClosed())
				})
			})

			Context("Firehose", func() {
				var (
					messages <-chan *events.Envelope
					errors   <-chan error
				)

				It("passes messages through for every app for uaa admins", func() {
					client := consumer.New(tcWSEndpoint, &tls.Config{
						InsecureSkipVerify: true,
					}, nil)
					defer client.Close()
					messages, errors = client.FirehoseWithoutReconnect(SUBSCRIPTION_ID, AUTH_TOKEN)

					var grpcRequest *plumbing.SubscriptionRequest
					Eventually(fakeDoppler.SubscriptionRequests, 10).Should(Receive(&grpcRequest))
					Expect(grpcRequest.ShardID).To(Equal(SUBSCRIPTION_ID))

					currentTime := time.Now().UnixNano()
					dropsondeMessage := makeDropsondeMessage("Hello through NOAA", APP_ID, currentTime)
					fakeDoppler.SendLogMessage(dropsondeMessage)

					var receivedEnvelope *events.Envelope
					Eventually(messages).Should(Receive(&receivedEnvelope))
					Consistently(errors).ShouldNot(Receive())

					Expect(receivedEnvelope.GetEventType()).To(Equal(events.Envelope_LogMessage))
					receivedMessage := receivedEnvelope.GetLogMessage()
					Expect(receivedMessage.GetMessage()).To(BeEquivalentTo("Hello through NOAA"))
					Expect(receivedMessage.GetAppId()).To(Equal(APP_ID))
					Expect(receivedMessage.GetTimestamp()).To(Equal(currentTime))
				})
			})

		})

		Describe("LogCache API Paths", func() {
			Context("Recent", func() {
				It("returns a multi-part HTTP response with all recent messages", func() {
					client := consumer.New(tcWSEndpoint, &tls.Config{
						InsecureSkipVerify: true,
					}, nil)

					Eventually(func() int {
						messages, err := client.RecentLogs("efe5c422-e8a7-42c2-a52b-98bffd8d6a07", "bearer iAmAnAdmin")
						Expect(err).NotTo(HaveOccurred())

						if len(logCache.requests()) > 0 {
							Expect(logCache.requests()[0].SourceId).To(Equal("efe5c422-e8a7-42c2-a52b-98bffd8d6a07"))
						}

						return len(messages)
					}, 5).Should(Equal(2))
				})
			})
		})

		Context("SetCookie", func() {
			It("sets the desired cookie on the response", func() {
				response, err := httpClient.PostForm(
					fmt.Sprintf("https://%s:%d/set-cookie",
						localIPAddress,
						wsPort,
					),
					url.Values{
						"CookieName":  {"authorization"},
						"CookieValue": {url.QueryEscape("bearer iAmAnAdmin")},
					},
				)
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Cookies()).NotTo(BeNil())
				Expect(response.Cookies()).To(HaveLen(1))
				cookie := response.Cookies()[0]
				Expect(cookie.Domain).To(Equal("doppler.vcap.me"))
				Expect(cookie.Name).To(Equal("authorization"))
				Expect(cookie.Value).To(Equal("bearer+iAmAnAdmin"))
				Expect(cookie.Secure).To(BeTrue())
			})
		})
	})
})
