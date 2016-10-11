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
	"sync"
	"time"
	"trafficcontroller/doppler_endpoint"
	"trafficcontroller/dopplerproxy"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"
	"github.com/gorilla/websocket"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServeHTTP()", func() {
	var (
		auth                  LogAuthorizer
		adminAuth             AdminAuthorizer
		proxy                 *dopplerproxy.Proxy
		recorder              *httptest.ResponseRecorder
		channelGroupConnector *fakeChannelGroupConnector

		mockGrpcConnector         *mockGrpcConnector
		mockDopplerStreamClient   *mockReceiver
		mockDopplerFirehoseClient *mockReceiver
	)

	BeforeEach(func() {
		auth = LogAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		adminAuth = AdminAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		mockGrpcConnector = newMockGrpcConnector()

		mockDopplerStreamClient = newMockReceiver()
		mockDopplerFirehoseClient = newMockReceiver()

		mockGrpcConnector.StreamOutput.Ret0 <- mockDopplerStreamClient
		mockGrpcConnector.FirehoseOutput.Ret0 <- mockDopplerFirehoseClient

		channelGroupConnector = &fakeChannelGroupConnector{messages: make(chan []byte, 10)}

		proxy = dopplerproxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			channelGroupConnector,
			mockGrpcConnector,
			"cookieDomain",
			loggertesthelper.Logger(),
		)

		recorder = httptest.NewRecorder()
	})

	JustBeforeEach(func() {
		close(mockGrpcConnector.StreamOutput.Ret0)
		close(mockGrpcConnector.StreamOutput.Ret1)

		close(mockGrpcConnector.FirehoseOutput.Ret0)
		close(mockGrpcConnector.FirehoseOutput.Ret1)

		close(mockDopplerStreamClient.RecvOutput.Ret0)
		close(mockDopplerStreamClient.RecvOutput.Ret1)

		close(mockDopplerFirehoseClient.RecvOutput.Ret0)
		close(mockDopplerFirehoseClient.RecvOutput.Ret1)
	})

	Context("App Logs", func() {

		Describe("TrafficController emitted request metrics", func() {
			var fakeMetricSender *fake.FakeMetricSender

			BeforeEach(func() {
				fakeMetricSender = fake.NewFakeMetricSender()
				metricBatcher := metricbatcher.New(fakeMetricSender, time.Millisecond)
				metrics.Initialize(fakeMetricSender, metricBatcher)
			})

			requestAndAssert := func(req *http.Request, metricName string) {
				requestStart := time.Now()
				proxy.ServeHTTP(recorder, req)
				metric := fakeMetricSender.GetValue(metricName)
				elapsed := float64(time.Since(requestStart)) / float64(time.Millisecond)

				Expect(metric.Unit).To(Equal("ms"))
				Expect(metric.Value).To(BeNumerically("<", elapsed))

			}

			It("Should emit value metric for recentlogs request", func() {
				close(channelGroupConnector.messages)
				req, _ := http.NewRequest("GET", "/apps/appID123/recentlogs", nil)
				requestAndAssert(req, "dopplerProxy.recentlogsLatency")
			})

			It("Should emit value metric for containermetrics request", func() {
				mockGrpcConnector.ContainerMetricsOutput.Ret0 <- new(plumbing.ContainerMetricsResponse)
				close(mockGrpcConnector.ContainerMetricsOutput.Ret1)

				req, _ := http.NewRequest("GET", "/apps/appID123/containermetrics", nil)
				requestAndAssert(req, "dopplerProxy.containermetricsLatency")
			})

			It("Should Not emit any metrics for stream request", func() {
				close(channelGroupConnector.messages)
				req, _ := http.NewRequest("GET", "/apps/appID123/stream", nil)
				proxy.ServeHTTP(recorder, req)
				metric := fakeMetricSender.GetValue("dopplerProxy.streamLatency")

				Expect(metric.Unit).To(BeEmpty())
				Expect(metric.Value).To(BeZero())
			})
		})

		Context("if the path does not end with /stream or /recentlogs", func() {
			It("returns a 404", func() {
				req, _ := http.NewRequest("GET", "/apps/abc123/bar", nil)

				proxy.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})

			It("It does not attempt to connect to doppler", func() {
				req, _ := http.NewRequest("GET", "/apps/abc123/bar", nil)

				proxy.ServeHTTP(recorder, req)
				Consistently(channelGroupConnector.getPath).Should(Equal(""))
				Consistently(channelGroupConnector.getStreamId).Should(Equal(""))
				Consistently(channelGroupConnector.getReconnect).Should(BeFalse())
			})
		})

		Context("if the app id is forbidden", func() {
			It("returns a not found status", func() {
				auth.Result = AuthorizerResult{Status: http.StatusForbidden, ErrorMessage: http.StatusText(http.StatusForbidden)}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(http.StatusNotFound))
				Expect(recorder.Body.String()).To(Equal(http.StatusText(http.StatusNotFound)))
			})
		})

		Context("if the app id is not found", func() {
			It("returns a not found status", func() {
				auth.Result = AuthorizerResult{Status: http.StatusNotFound, ErrorMessage: http.StatusText(http.StatusNotFound)}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(http.StatusNotFound))
				Expect(recorder.Body.String()).To(Equal(http.StatusText(http.StatusNotFound)))
			})
		})

		Context("if any other error occurs", func() {
			It("returns an Internal Server Error", func() {
				auth.Result = AuthorizerResult{Status: http.StatusInternalServerError, ErrorMessage: "some bad error"}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
				Expect(recorder.Body.String()).To(Equal(http.StatusText(http.StatusInternalServerError)))
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
				Expect(recorder.Body.String()).To(Equal(http.StatusText(http.StatusUnauthorized)))
			})

			It("does not attempt to connect to doppler", func() {
				auth.Result = AuthorizerResult{Status: http.StatusUnauthorized, ErrorMessage: "Authorization Failed"}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Consistently(mockGrpcConnector.StreamCalled).ShouldNot(Receive())
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

			Eventually(mockGrpcConnector.StreamCalled).Should(Receive())

			Expect(mockGrpcConnector.StreamInput.Ctx).To(Receive(Not(BeNil())))
			Expect(mockGrpcConnector.StreamInput.In).To(Receive(Equal(&plumbing.StreamRequest{"abc123"})))
		})

		It("connects to doppler servers without reconnecting for recentlogs", func() {
			close(channelGroupConnector.messages)
			req, _ := http.NewRequest("GET", "/apps/abc123/recentlogs", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(channelGroupConnector.getReconnect).Should(BeFalse())
		})

		It("connects to doppler servers without reconnecting for containermetrics", func() {
			mockGrpcConnector.ContainerMetricsOutput.Ret0 <- new(plumbing.ContainerMetricsResponse)
			close(mockGrpcConnector.ContainerMetricsOutput.Ret1)
			req, _ := http.NewRequest("GET", "/apps/abc123/containermetrics", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(channelGroupConnector.getReconnect).Should(BeFalse())
		})

		It("passes messages back to the requestor", func() {
			channelGroupConnector.messages <- []byte("hello")
			channelGroupConnector.messages <- []byte("goodbye")
			close(channelGroupConnector.messages)

			req, _ := http.NewRequest("GET", "/apps/abc123/recentlogs", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			responseBody, _ := ioutil.ReadAll(recorder.Body)

			Expect(responseBody).To(ContainSubstring("hello"))
			Expect(responseBody).To(ContainSubstring("goodbye"))
		})

		It("stops the channel connector when the client closes its connection", func() {
			close(channelGroupConnector.messages)
			req, _ := http.NewRequest("GET", "/apps/abc123/recentlogs", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(channelGroupConnector.Stopped).Should(BeTrue())
		})

		It("stops the connector when the client closes its connection", func() {
			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			var ctx context.Context
			Eventually(mockGrpcConnector.StreamInput.Ctx).Should(Receive(&ctx))
			Eventually(ctx.Done).Should(BeClosed())
		})

		It("returns the requested container metrics", func(done Done) {
			defer close(done)
			req, _ := http.NewRequest("GET", "/apps/abc123/containermetrics", nil)
			req.Header.Add("Authorization", "token")
			containerResp := &plumbing.ContainerMetricsResponse{
				Payload: [][]byte{
					[]byte("foo"),
					[]byte("bar"),
					[]byte("baz"),
				},
			}
			mockGrpcConnector.ContainerMetricsOutput.Ret0 <- containerResp
			mockGrpcConnector.ContainerMetricsOutput.Ret1 <- nil

			proxy.ServeHTTP(recorder, req)

			boundaryRegexp := regexp.MustCompile("boundary=(.*)")
			matches := boundaryRegexp.FindStringSubmatch(recorder.Header().Get("Content-Type"))
			Expect(matches).To(HaveLen(2))
			Expect(matches[1]).NotTo(BeEmpty())
			reader := multipart.NewReader(recorder.Body, matches[1])

			for _, payload := range containerResp.Payload {
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

				expectedRequest := &plumbing.FirehoseRequest{
					SubID: "abc-123",
				}
				Eventually(mockGrpcConnector.FirehoseInput.In).Should(BeCalled(With(expectedRequest)))
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
					mockDopplerStreamClient.RecvOutput.Ret0 <- &plumbing.Response{
						Payload: expectedData,
					}

					mockDopplerFirehoseClient.RecvOutput.Ret0 <- &plumbing.Response{
						Payload: expectedData,
					}
				})

				It("/stream sends data to the client websocket connection", func(done Done) {
					defer close(done)
					conn, _, err := websocket.DefaultDialer.Dial(
						wsEndpoint("/apps/abc123/stream"),
						http.Header{"Authorization": []string{"token"}},
					)
					Expect(err).ToNot(HaveOccurred())

					_, data, err := conn.ReadMessage()
					Expect(err).ToNot(HaveOccurred())

					Expect(data).To(Equal(expectedData))
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
				})

				Context("with doppler's stream endpoint returning an error", func() {
					BeforeEach(func() {
						mockGrpcConnector.StreamOutput.Ret1 <- fmt.Errorf("some-error")
					})

					It("writes a 500", func(done Done) {
						defer close(done)
						_, resp, err := websocket.DefaultDialer.Dial(
							wsEndpoint("/apps/abc123/stream"),
							http.Header{"Authorization": []string{"token"}},
						)
						Expect(err).To(HaveOccurred())

						Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("with doppler's firehose endpoint returning an error", func() {
					BeforeEach(func() {
						mockGrpcConnector.FirehoseOutput.Ret1 <- fmt.Errorf("some-error")
					})

					It("writes a 500", func(done Done) {
						defer close(done)
						_, resp, err := websocket.DefaultDialer.Dial(
							wsEndpoint("/firehose/abc123"),
							http.Header{"Authorization": []string{"token"}},
						)
						Expect(err).To(HaveOccurred())

						Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("with GRPC recv returning an error", func() {
				BeforeEach(func() {
					mockDopplerFirehoseClient.RecvOutput.Ret1 <- errors.New("foo")
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

type fakeChannelGroupConnector struct {
	messages        chan []byte
	dopplerEndpoint doppler_endpoint.DopplerEndpoint
	stopped         bool
	sync.Mutex
}

func (f *fakeChannelGroupConnector) Connect(dopplerEndpoint doppler_endpoint.DopplerEndpoint, messagesChan chan<- []byte, stopChan <-chan struct{}) {

	go func() {
		for m := range f.messages {
			messagesChan <- m
		}
		close(messagesChan)
	}()

	go func() {
		<-stopChan
		f.Lock()
		defer f.Unlock()
		f.stopped = true
	}()

	f.Lock()
	defer f.Unlock()
	f.dopplerEndpoint = dopplerEndpoint
}

func (f *fakeChannelGroupConnector) getPath() string {
	f.Lock()
	defer f.Unlock()
	return f.dopplerEndpoint.Endpoint
}

func (f *fakeChannelGroupConnector) getStreamId() string {
	f.Lock()
	defer f.Unlock()
	return f.dopplerEndpoint.StreamId
}

func (f *fakeChannelGroupConnector) getReconnect() bool {
	f.Lock()
	defer f.Unlock()
	return f.dopplerEndpoint.Reconnect
}

func (f *fakeChannelGroupConnector) Stopped() bool {
	f.Lock()
	defer f.Unlock()
	return f.stopped
}
