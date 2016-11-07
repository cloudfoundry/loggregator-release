//go:generate hel

package dopplerproxy_test

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"plumbing"
	"regexp"
	"strings"
	"time"
	"trafficcontroller/doppler_endpoint"
	"trafficcontroller/dopplerproxy"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServeHTTP()", func() {
	var (
		auth      LogAuthorizer
		adminAuth AdminAuthorizer
		proxy     *dopplerproxy.Proxy
		recorder  *httptest.ResponseRecorder

		mockGrpcConnector       *mockGrpcConnector
		mockDopplerStreamClient *mockReceiver
		fakeMetricSender        *fake.FakeMetricSender
	)

	BeforeSuite(func() {
		fakeMetricSender = fake.NewFakeMetricSender()
		metricBatcher := metricbatcher.New(fakeMetricSender, time.Millisecond)
		metrics.Initialize(fakeMetricSender, metricBatcher)
	})

	BeforeEach(func() {
		auth = LogAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		adminAuth = AdminAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		mockGrpcConnector = newMockGrpcConnector()

		mockDopplerStreamClient = newMockReceiver()

		mockGrpcConnector.SubscribeOutput.Ret0 <- mockDopplerStreamClient

		proxy = dopplerproxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			mockGrpcConnector,
			"cookieDomain",
			loggertesthelper.Logger(),
			50*time.Millisecond,
		)

		recorder = httptest.NewRecorder()

		fakeMetricSender.Reset()
	})

	JustBeforeEach(func() {
		close(mockGrpcConnector.SubscribeOutput.Ret0)
		close(mockGrpcConnector.SubscribeOutput.Ret1)

		close(mockDopplerStreamClient.RecvOutput.Ret0)
		close(mockDopplerStreamClient.RecvOutput.Ret1)
	})

	Context("App Logs", func() {
		Describe("TrafficController emitted request metrics", func() {
			requestAndAssert := func(req *http.Request, metricName string) {
				requestStart := time.Now()
				proxy.ServeHTTP(recorder, req)
				metric := fakeMetricSender.GetValue(metricName)
				elapsed := float64(time.Since(requestStart)) / float64(time.Millisecond)

				Expect(metric.Unit).To(Equal("ms"))
				Expect(metric.Value).To(BeNumerically("<", elapsed))
			}

			It("emits latency value metric for recentlogs request", func() {
				mockGrpcConnector.RecentLogsOutput.Ret0 <- nil
				req, _ := http.NewRequest("GET", "/apps/appID123/recentlogs", nil)
				requestAndAssert(req, "dopplerProxy.recentlogsLatency")
			})

			It("emits latency value metric for containermetrics request", func() {
				mockGrpcConnector.ContainerMetricsOutput.Ret0 <- nil

				req, _ := http.NewRequest("GET", "/apps/appID123/containermetrics", nil)
				requestAndAssert(req, "dopplerProxy.containermetricsLatency")
			})

			It("does not emit any latency metrics for stream request", func() {
				req, _ := http.NewRequest("GET", "/apps/appID123/stream", nil)
				proxy.ServeHTTP(recorder, req)
				metric := fakeMetricSender.GetValue("dopplerProxy.streamLatency")

				Expect(metric.Unit).To(BeEmpty())
				Expect(metric.Value).To(BeZero())
			})

			It("creates a context with a deadline for recent logs", func() {
				go func() {
					time.Sleep(100 * time.Millisecond)
					mockGrpcConnector.RecentLogsOutput.Ret0 <- nil
				}()
				req, _ := http.NewRequest("GET", "/apps/appID123/recentlogs", nil)
				proxy.ServeHTTP(recorder, req)

				var ctx context.Context
				Eventually(mockGrpcConnector.RecentLogsInput.Ctx).Should(Receive(&ctx))
				_, ok := ctx.Deadline()
				Expect(ok).To(BeTrue())

				Eventually(ctx.Err).Should(HaveOccurred())
				Expect(recorder.Code).To(Equal(http.StatusServiceUnavailable))
			})

			It("creates a context with a deadline for container metrics", func() {
				go func() {
					time.Sleep(100 * time.Millisecond)
					mockGrpcConnector.ContainerMetricsOutput.Ret0 <- nil
				}()
				req, _ := http.NewRequest("GET", "/apps/appID123/containermetrics", nil)
				proxy.ServeHTTP(recorder, req)

				var ctx context.Context
				Eventually(mockGrpcConnector.ContainerMetricsInput.Ctx).Should(Receive(&ctx))
				_, ok := ctx.Deadline()
				Expect(ok).To(BeTrue())

				Eventually(ctx.Err).Should(HaveOccurred())
				Expect(recorder.Code).To(Equal(http.StatusServiceUnavailable))
			})
		})

		Context("if the path is not valid", func() {
			It("returns a 404", func() {
				req, _ := http.NewRequest("GET", "/apps/abc123/bar", nil)

				proxy.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
		})

		Context("if the app id is forbidden", func() {
			It("returns a not found status", func() {
				auth.Result = AuthorizerResult{Status: http.StatusForbidden, ErrorMessage: http.StatusText(http.StatusForbidden)}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
		})

		Context("if the app id is not found", func() {
			It("returns a not found status", func() {
				auth.Result = AuthorizerResult{Status: http.StatusNotFound, ErrorMessage: http.StatusText(http.StatusNotFound)}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
		})

		Context("if any other error occurs", func() {
			It("returns an Internal Server Error", func() {
				auth.Result = AuthorizerResult{Status: http.StatusInternalServerError, ErrorMessage: "some bad error"}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("if authorization fails", func() {
			It("returns an unauthorized status and sets the WWW-Authenticate header", func() {
				auth.Result = AuthorizerResult{Status: http.StatusUnauthorized, ErrorMessage: "Error: Invalid authorization"}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Expect(auth.TokenParam).To(Equal("token"))
				Expect(auth.Target).To(Equal("abc123"))

				Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
			})

			It("does not attempt to connect to doppler", func() {
				auth.Result = AuthorizerResult{Status: http.StatusUnauthorized, ErrorMessage: "Authorization Failed"}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Consistently(mockGrpcConnector.SubscribeCalled).ShouldNot(Receive())
			})
		})

		It("can read the authorization information from a cookie", func() {
			auth.Result = AuthorizerResult{Status: http.StatusUnauthorized, ErrorMessage: "Authorization Failed"}

			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)

			req.AddCookie(&http.Cookie{Name: "authorization", Value: "cookie-token"})

			proxy.ServeHTTP(recorder, req)

			Expect(auth.TokenParam).To(Equal("cookie-token"))
		})

		It("connects to doppler servers with correct parameters", func() {
			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(mockGrpcConnector.SubscribeCalled).Should(Receive())

			Expect(mockGrpcConnector.SubscribeInput.Ctx).To(Receive(Not(BeNil())))
			Expect(mockGrpcConnector.SubscribeInput.Req).To(Receive(Equal(
				&plumbing.SubscriptionRequest{
					Filter: &plumbing.Filter{
						AppID: "abc123",
					},
				},
			)))
		})

		It("closes the context when the client closes its connection", func() {
			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			var ctx context.Context
			Eventually(mockGrpcConnector.SubscribeInput.Ctx).Should(Receive(&ctx))
			Eventually(ctx.Done).Should(BeClosed())
		})

		It("returns the requested container metrics", func(done Done) {
			defer close(done)
			req, _ := http.NewRequest("GET", "/apps/abc123/containermetrics", nil)
			req.Header.Add("Authorization", "token")
			now := time.Now()
			_, envBytes1 := buildContainerMetric("abc123", now)
			_, envBytes2 := buildContainerMetric("abc123", now.Add(-5*time.Minute))
			containerResp := [][]byte{
				envBytes1,
				envBytes2,
			}
			mockGrpcConnector.ContainerMetricsOutput.Ret0 <- containerResp

			proxy.ServeHTTP(recorder, req)

			boundaryRegexp := regexp.MustCompile("boundary=(.*)")
			matches := boundaryRegexp.FindStringSubmatch(recorder.Header().Get("Content-Type"))
			Expect(matches).To(HaveLen(2))
			Expect(matches[1]).NotTo(BeEmpty())
			reader := multipart.NewReader(recorder.Body, matches[1])

			part, err := reader.NextPart()
			Expect(err).ToNot(HaveOccurred())

			partBytes, err := ioutil.ReadAll(part)
			Expect(err).ToNot(HaveOccurred())
			Expect(partBytes).To(Equal(containerResp[0]))
		})

		It("returns the requested recent logs", func() {
			req, _ := http.NewRequest("GET", "/apps/abc123/recentlogs", nil)
			req.Header.Add("Authorization", "token")
			recentLogResp := [][]byte{
				[]byte("log1"),
				[]byte("log2"),
				[]byte("log3"),
			}
			mockGrpcConnector.RecentLogsOutput.Ret0 <- recentLogResp

			proxy.ServeHTTP(recorder, req)

			boundaryRegexp := regexp.MustCompile("boundary=(.*)")
			matches := boundaryRegexp.FindStringSubmatch(recorder.Header().Get("Content-Type"))
			Expect(matches).To(HaveLen(2))
			Expect(matches[1]).NotTo(BeEmpty())
			reader := multipart.NewReader(recorder.Body, matches[1])

			for _, payload := range recentLogResp {
				part, err := reader.NextPart()
				Expect(err).ToNot(HaveOccurred())

				partBytes, err := ioutil.ReadAll(part)
				Expect(err).ToNot(HaveOccurred())
				Expect(partBytes).To(Equal(payload))
			}
		})
	})

	Context("Firehose", func() {
		Context("if a subscription_id is provided", func() {
			It("connects to doppler servers with correct parameters", func() {
				req, _ := http.NewRequest("GET", "/firehose/abc-123", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				expectedRequest := &plumbing.SubscriptionRequest{
					ShardID: "abc-123",
				}
				Eventually(mockGrpcConnector.SubscribeInput.Req).Should(BeCalled(With(expectedRequest)))
			})

			It("returns an unauthorized status and sets the WWW-Authenticate header if authorization fails", func() {
				adminAuth.Result = AuthorizerResult{Status: http.StatusUnauthorized, ErrorMessage: "Error: Invalid authorization"}

				req, _ := http.NewRequest("GET", "/firehose/abc-123", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Expect(adminAuth.TokenParam).To(Equal("token"))

				Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
				Expect(recorder.Body.String()).To(Equal("You are not authorized. Error: Invalid authorization"))
			})
		})

		Context("if subscription_id is not provided", func() {
			It("returns a 404", func() {
				req, _ := http.NewRequest("GET", "/firehose/", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
		})
	})

	Context("Other invalid paths", func() {
		It("returns a 404 for an empty path", func() {
			req, _ := http.NewRequest("GET", "/", nil)
			proxy.ServeHTTP(recorder, req)
			Expect(recorder.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("SetCookie", func() {
		It("returns an OK status with a form", func() {
			req, _ := http.NewRequest("POST", "/set-cookie", strings.NewReader("CookieName=cookie&CookieValue=monster"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			proxy.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("sets the passed value as a cookie", func() {
			req, _ := http.NewRequest("POST", "/set-cookie", strings.NewReader("CookieName=cookie&CookieValue=monster"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			proxy.ServeHTTP(recorder, req)

			Expect(recorder.Header().Get("Set-Cookie")).To(Equal("cookie=monster; Domain=cookieDomain; Secure"))
		})

		It("returns a bad request if the form does not parse", func() {
			req, _ := http.NewRequest("POST", "/set-cookie", nil)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			proxy.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
		})

		It("sets required CORS headers", func() {
			req, _ := http.NewRequest("POST", "/set-cookie", nil)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", "fake-origin-string")

			proxy.ServeHTTP(recorder, req)

			Expect(recorder.Header().Get("Access-Control-Allow-Origin")).To(Equal("fake-origin-string"))
			Expect(recorder.Header().Get("Access-Control-Allow-Credentials")).To(Equal("true"))
		})
	})

	Describe("Streaming Data", func() {
		var (
			server *httptest.Server
		)

		var wsEndpoint = func(path string) string {
			return strings.Replace(server.URL, "http", "ws", 1) + path
		}

		BeforeEach(func() {
			server = httptest.NewServer(proxy)
		})

		AfterEach(func() {
			server.CloseClientConnections()
		})

		Describe("/stream & /firehose", func() {
			Context("with GRPC recv returning data", func() {
				var (
					expectedData []byte
				)

				BeforeEach(func() {
					expectedData = []byte("hello")
					mockDopplerStreamClient.RecvOutput.Ret0 <- expectedData
				})

				It("/stream sends data to the client websocket connection", func() {
					conn, _, err := websocket.DefaultDialer.Dial(
						wsEndpoint("/apps/abc123/stream"),
						http.Header{"Authorization": []string{"token"}},
					)
					Expect(err).ToNot(HaveOccurred())

					_, data, err := conn.ReadMessage()
					Expect(err).ToNot(HaveOccurred())

					Expect(data).To(Equal(expectedData))

					var ctx context.Context
					Eventually(mockGrpcConnector.SubscribeInput.Ctx).Should(Receive(&ctx))
					_, ok := ctx.Deadline()
					Expect(ok).To(BeFalse())
				})

				It("/firehose sends data to the client websocket connection", func(done Done) {
					defer close(done)
					conn, _, err := websocket.DefaultDialer.Dial(
						wsEndpoint("/firehose/subscription-id"),
						http.Header{"Authorization": []string{"token"}},
					)
					Expect(err).ToNot(HaveOccurred())

					_, data, err := conn.ReadMessage()
					Expect(err).ToNot(HaveOccurred())

					Expect(data).To(Equal(expectedData))

					var ctx context.Context
					Eventually(mockGrpcConnector.SubscribeInput.Ctx).Should(Receive(&ctx))
					_, ok := ctx.Deadline()
					Expect(ok).To(BeFalse())
				})
			})

			Context("with GRPC recv returning an error", func() {
				BeforeEach(func() {
					mockDopplerStreamClient.RecvOutput.Ret1 <- errors.New("foo")
				})

				It("closes the connection to the client", func(done Done) {
					defer close(done)
					conn, _, err := websocket.DefaultDialer.Dial(
						wsEndpoint("/firehose/subscription-id"),
						http.Header{"Authorization": []string{"token"}},
					)
					Expect(err).ToNot(HaveOccurred())

					f := func() string {
						_, _, err := conn.ReadMessage()
						return fmt.Sprintf("%s", err)
					}
					Eventually(f).Should(ContainSubstring("websocket: close 1000"))
				})
			})

			Describe("Emitted Metrics", func() {
				It("emits a metric saying we have subscriptions", func() {
					conn, _, err := websocket.DefaultDialer.Dial(
						wsEndpoint("/firehose/subscription-id"),
						http.Header{"Authorization": []string{"token"}},
					)
					Expect(err).ToNot(HaveOccurred())
					defer conn.Close()

					f := func() fake.Metric {
						return fakeMetricSender.GetValue("dopplerProxy.firehoses")
					}
					Eventually(f, 4).Should(Equal(fake.Metric{Value: 1, Unit: "connections"}))
				})

				It("emits a metric saying we have a app stream", func() {
					conn, _, err := websocket.DefaultDialer.Dial(
						wsEndpoint("/apps/abc123/stream"),
						http.Header{"Authorization": []string{"token"}},
					)
					Expect(err).ToNot(HaveOccurred())
					defer conn.Close()

					f := func() fake.Metric {
						return fakeMetricSender.GetValue("dopplerProxy.appStreams")
					}
					Eventually(f, 4).Should(Equal(fake.Metric{Value: 1, Unit: "connections"}))
				})
			})
		})
	})
})

var _ = Describe("DefaultHandlerProvider", func() {
	It("returns an HTTP handler for .../recentlogs", func() {
		httpHandler := handlers.NewHttpHandler(make(chan []byte), loggertesthelper.Logger())

		target := doppler_endpoint.HttpHandlerProvider(make(chan []byte), loggertesthelper.Logger())

		Expect(target).To(BeAssignableToTypeOf(httpHandler))
	})

	It("returns a Websocket handler for .../stream", func() {
		wsHandler := handlers.NewWebsocketHandler(make(chan []byte), time.Minute, loggertesthelper.Logger())

		target := doppler_endpoint.WebsocketHandlerProvider(make(chan []byte), loggertesthelper.Logger())

		Expect(target).To(BeAssignableToTypeOf(wsHandler))
	})

	It("returns a Websocket handler for anything else", func() {
		wsHandler := handlers.NewWebsocketHandler(make(chan []byte), time.Minute, loggertesthelper.Logger())

		target := doppler_endpoint.WebsocketHandlerProvider(make(chan []byte), loggertesthelper.Logger())

		Expect(target).To(BeAssignableToTypeOf(wsHandler))
	})
})

func buildContainerMetric(appID string, t time.Time) (*events.Envelope, []byte) {
	envelope := &events.Envelope{
		Origin:    proto.String("doppler"),
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(t.UnixNano()),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String(appID),
			InstanceIndex: proto.Int32(int32(1)),
			CpuPercentage: proto.Float64(float64(1)),
			MemoryBytes:   proto.Uint64(uint64(1)),
			DiskBytes:     proto.Uint64(uint64(1)),
		},
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}
