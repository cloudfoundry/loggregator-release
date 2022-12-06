package trafficcontroller_test

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/tlsconfig/certtest"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/testservers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/noaa/v2/consumer"
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
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		}
	)

	Context("with a configured log cache", func() {
		Context("when log cache is available", func() {
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
							InsecureSkipVerify: true, //nolint:gosec
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
							InsecureSkipVerify: true, //nolint:gosec
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

			Describe("TLS security", func() {
				DescribeTable("allows only supported TLS versions", func(clientTLSVersion int, serverShouldAllow bool) {
					tlsConfig := buildTLSConfig(uint16(clientTLSVersion), tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256)
					client := consumer.New(tcWSEndpoint, tlsConfig, nil)
					_, errors := client.StreamWithoutReconnect(APP_ID, AUTH_TOKEN)

					defer client.Close()

					if serverShouldAllow {
						Consistently(errors).ShouldNot(Receive())
					} else {
						Eventually(errors).Should(Receive())
					}
				},
					Entry("unsupported SSL 3.0", tls.VersionSSL30, false), //nolint:staticcheck
					Entry("unsupported TLS 1.0", tls.VersionTLS10, false),
					Entry("unsupported TLS 1.1", tls.VersionTLS11, false),
					Entry("supported TLS 1.2", tls.VersionTLS12, true),
				)

				DescribeTable("allows only supported TLS versions", func(cipherSuite uint16, serverShouldAllow bool) {
					tlsConfig := buildTLSConfig(tls.VersionTLS12, cipherSuite)
					client := consumer.New(tcWSEndpoint, tlsConfig, nil)
					_, errors := client.StreamWithoutReconnect(APP_ID, AUTH_TOKEN)

					defer client.Close()

					if serverShouldAllow {
						Consistently(errors).ShouldNot(Receive())
					} else {
						Eventually(errors).Should(Receive())
					}
				},
					Entry("unsupported cipher RSA_WITH_3DES_EDE_CBC_SHA", tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA, false),
					Entry("unsupported cipher ECDHE_RSA_WITH_3DES_EDE_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA, false),
					Entry("unsupported cipher RSA_WITH_RC4_128_SHA", tls.TLS_RSA_WITH_RC4_128_SHA, false),
					Entry("unsupported cipher RSA_WITH_AES_128_CBC_SHA256", tls.TLS_RSA_WITH_AES_128_CBC_SHA256, false),
					Entry("unsupported cipher ECDHE_ECDSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, false),
					Entry("unsupported cipher ECDHE_ECDSA_WITH_RC4_128_SHA", tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, false),
					Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_CBC_SHA", tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, false),
					Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_256_CBC_SHA", tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, false),
					Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_CBC_SHA256", tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, false),
					Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, false),
					Entry("unsupported cipher ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, false),
					Entry("unsupported cipher ECDHE_RSA_WITH_RC4_128_SHA", tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA, false),
					Entry("unsupported cipher ECDHE_RSA_WITH_AES_128_CBC_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256, false),
					Entry("unsupported cipher ECDHE_RSA_WITH_AES_128_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA, false),
					Entry("unsupported cipher ECDHE_RSA_WITH_AES_256_CBC_SHA", tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA, false),
					Entry("unsupported cipher ECDHE_RSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305, false),
					Entry("unsupported cipher RSA_WITH_AES_128_CBC_SHA", tls.TLS_RSA_WITH_AES_128_CBC_SHA, false),
					Entry("unsupported cipher RSA_WITH_AES_128_GCM_SHA256", tls.TLS_RSA_WITH_AES_128_GCM_SHA256, false),
					Entry("unsupported cipher RSA_WITH_AES_256_CBC_SHA", tls.TLS_RSA_WITH_AES_256_CBC_SHA, false),
					Entry("unsupported cipher RSA_WITH_AES_256_GCM_SHA384", tls.TLS_RSA_WITH_AES_256_GCM_SHA384, false),

					Entry("supported cipher ECDHE_RSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, true),
					Entry("supported cipher ECDHE_RSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, true),
				)
			})

		})
		Context("when log cache is unavailable", func() {
			var (
				client   *consumer.Consumer
				messages <-chan *events.Envelope
				errors   <-chan error
			)

			BeforeEach(func() {
				fakeDoppler = NewFakeDoppler()
				err := fakeDoppler.Start()
				Expect(err).ToNot(HaveOccurred())

				lis, err := net.Listen("tcp", "127.0.0.1:0")
				Expect(err).ToNot(HaveOccurred())

				err = lis.Close()
				Expect(err).ToNot(HaveOccurred())

				cfg := testservers.BuildTrafficControllerConf(fakeDoppler.Addr(), 37474, lis.Addr().String())

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

				client = consumer.New(tcWSEndpoint, &tls.Config{
					InsecureSkipVerify: true, //nolint:gosec
				}, nil)
				messages, errors = client.StreamWithoutReconnect(APP_ID, AUTH_TOKEN)
			})

			AfterEach(func(done Done) {
				defer close(done)
				fakeDoppler.Stop()
			}, 30)

			It("still passes messages through", func() {
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

		})

	})
})

func buildTLSConfig(maxVersion, cipherSuite uint16) *tls.Config {
	ca, err := certtest.BuildCA("tlsconfig")
	Expect(err).ToNot(HaveOccurred())

	pool, err := ca.CertPool()
	Expect(err).ToNot(HaveOccurred())

	clientCrt, err := ca.BuildSignedCertificate("client")
	Expect(err).ToNot(HaveOccurred())

	clientTLSCrt, err := clientCrt.TLSCertificate()
	Expect(err).ToNot(HaveOccurred())

	return &tls.Config{
		Certificates:       []tls.Certificate{clientTLSCrt},
		RootCAs:            pool,
		ServerName:         "",
		MaxVersion:         uint16(maxVersion),
		CipherSuites:       []uint16{cipherSuite},
		InsecureSkipVerify: true, //nolint:gosec
	}
}
