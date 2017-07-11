package trafficcontroller_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/noaa/consumer"
	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("TrafficController for dropsonde messages", func() {
	var dropsondeEndpoint string

	BeforeEach(func() {
		fakeDoppler = NewFakeDoppler()
		go fakeDoppler.Start()
		dropsondeEndpoint = fmt.Sprintf("ws://%s:%d", localIPAddress, TRAFFIC_CONTROLLER_DROPSONDE_PORT)
	})

	AfterEach(func() {
		fakeDoppler.Stop()
	})

	Context("Streaming", func() {
		var (
			client   *consumer.Consumer
			messages <-chan *events.Envelope
			errors   <-chan error
		)

		JustBeforeEach(func() {
			client = consumer.New(dropsondeEndpoint, &tls.Config{}, nil)
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
			var server plumbing.Doppler_SubscribeServer
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
			client := consumer.New(dropsondeEndpoint, &tls.Config{}, nil)
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

		It("returns a multi-part HTTP response with all recent messages", func() {
			client := consumer.New(dropsondeEndpoint, &tls.Config{}, nil)

			Eventually(func() bool {
				messages, err := client.RecentLogs("1234", "bearer iAmAnAdmin")
				Expect(err).NotTo(HaveOccurred())
				select {
				case request := <-fakeDoppler.RecentLogsRequests:
					Expect(request.AppID).To(Equal("1234"))
					Expect(messages).To(HaveLen(5))
					for i, message := range messages {
						Expect(message.GetMessage()).To(BeEquivalentTo(strconv.Itoa(i)))
					}
					return true
				default:
					return false
				}
			}, 5).Should(BeTrue())
		})
	})

	Context("ContainerMetrics", func() {
		BeforeEach(func() {
			for i := 0; i < 5; i++ {
				message := makeContainerMetricMessage("appID", i, i, i, i, 100000)
				fakeDoppler.SendLogMessage(message)
			}

			oldmessage := makeContainerMetricMessage("appID", 1, 6, 7, 8, 50000)
			fakeDoppler.SendLogMessage(oldmessage)

			fakeDoppler.CloseLogMessageStream()
		})

		It("returns a multi-part HTTP response with the most recent container metrics for all instances for a given app", func() {
			client := consumer.New(dropsondeEndpoint, &tls.Config{}, nil)

			Eventually(func() bool {
				messages, err := client.ContainerMetrics("1234", "bearer iAmAnAdmin")
				Expect(err).NotTo(HaveOccurred())

				select {
				case request := <-fakeDoppler.ContainerMetricsRequests:
					Expect(request.AppID).To(Equal("1234"))
					Expect(messages).To(HaveLen(5))
					for i, message := range messages {
						Expect(message.GetInstanceIndex()).To(BeEquivalentTo(i))
						Expect(message.GetCpuPercentage()).To(BeEquivalentTo(i))
					}
					return true
				default:
					return false
				}
			}, 5).Should(BeTrue())
		})
	})

	Context("SetCookie", func() {
		It("sets the desired cookie on the response", func() {
			response, err := http.PostForm(fmt.Sprintf("http://%s:%d/set-cookie", localIPAddress, TRAFFIC_CONTROLLER_DROPSONDE_PORT), url.Values{"CookieName": {"authorization"}, "CookieValue": {url.QueryEscape("bearer iAmAnAdmin")}})
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
