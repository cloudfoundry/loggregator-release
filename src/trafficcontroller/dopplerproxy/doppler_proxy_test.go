package dopplerproxy_test

import (
	"trafficcontroller/dopplerproxy"

	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
	"trafficcontroller/doppler_endpoint"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/server/handlers"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServeHTTP", func() {
	var (
		auth                  LogAuthorizer
		adminAuth             AdminAuthorizer
		proxy                 *dopplerproxy.Proxy
		recorder              *httptest.ResponseRecorder
		channelGroupConnector *fakeChannelGroupConnector
	)

	BeforeEach(func() {
		auth = LogAuthorizer{Result: AuthorizerResult{Authorized: true}}
		adminAuth = AdminAuthorizer{Result: AuthorizerResult{Authorized: true}}

		channelGroupConnector = &fakeChannelGroupConnector{messages: make(chan []byte, 10)}

		proxy = dopplerproxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			channelGroupConnector,
			dopplerproxy.TranslateFromDropsondePath,
			"cookieDomain",
			loggertesthelper.Logger(),
		)

		recorder = httptest.NewRecorder()
	})

	Context("App Logs", func() {
		It("returns a 200 for a head request", func() {
			req, _ := http.NewRequest("HEAD", "", nil)

			proxy.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		Context("metrices", func() {
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
				close(channelGroupConnector.messages)
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
				Expect(recorder.Body.String()).To(Equal("Resource Not Found."))
			})

			It("It does not attempt to connect to doppler", func() {
				req, _ := http.NewRequest("GET", "/apps/abc123/bar", nil)

				proxy.ServeHTTP(recorder, req)
				Consistently(channelGroupConnector.getPath).Should(Equal(""))
				Consistently(channelGroupConnector.getStreamId).Should(Equal(""))
				Consistently(channelGroupConnector.getReconnect).Should(BeFalse())
			})
		})

		Context("if the app id is missing", func() {
			It("returns a 404", func() {
				req, _ := http.NewRequest("GET", "/apps//stream", nil)

				proxy.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(http.StatusNotFound))
				Expect(recorder.Body.String()).To(Equal("App ID missing. Make request to /apps/APP_ID/stream"))
			})

			It("It does not attempt to connect to doppler", func() {
				req, _ := http.NewRequest("GET", "/apps//stream", nil)

				proxy.ServeHTTP(recorder, req)
				Consistently(channelGroupConnector.getPath).Should(Equal(""))
				Consistently(channelGroupConnector.getStreamId).Should(Equal(""))
				Consistently(channelGroupConnector.getReconnect).Should(BeFalse())
			})
		})

		Context("if authorization fails", func() {
			It("returns an unauthorized status and sets the WWW-Authenticate header", func() {
				auth.Result = AuthorizerResult{Authorized: false, ErrorMessage: "Error: Invalid authorization"}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Expect(auth.TokenParam).To(Equal("token"))
				Expect(auth.Target).To(Equal("abc123"))

				Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
				Expect(recorder.Body.String()).To(Equal("You are not authorized. Error: Invalid authorization"))
			})

			It("It does not attempt to connect to doppler", func() {
				auth.Result = AuthorizerResult{Authorized: false, ErrorMessage: "Authorization Failed"}

				req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Consistently(channelGroupConnector.getPath).Should(Equal(""))
				Consistently(channelGroupConnector.getStreamId).Should(Equal(""))
				Consistently(channelGroupConnector.getReconnect).Should(BeFalse())
			})
		})

		It("can read the authorization information from a cookie", func() {
			auth.Result = AuthorizerResult{Authorized: false, ErrorMessage: "Authorization Failed"}

			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)

			req.AddCookie(&http.Cookie{Name: "authorization", Value: "cookie-token"})

			proxy.ServeHTTP(recorder, req)

			Expect(auth.TokenParam).To(Equal("cookie-token"))
		})

		It("connects to doppler servers with correct parameters", func() {
			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(channelGroupConnector.getPath).Should(Equal("stream"))
			Eventually(channelGroupConnector.getReconnect).Should(BeTrue())
		})

		It("connects to doppler servers without reconnecting for recentlogs", func() {
			close(channelGroupConnector.messages)
			req, _ := http.NewRequest("GET", "/apps/abc123/recentlogs", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(channelGroupConnector.getReconnect).Should(BeFalse())
		})

		It("connects to doppler servers without reconnecting for containermetrics", func() {
			close(channelGroupConnector.messages)
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

		It("stops the connector when the handler finishes", func() {
			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			proxy.ServeHTTP(recorder, req)

			Eventually(channelGroupConnector.Stopped).Should(BeTrue())
		})
	})

	Context("Firehose", func() {
		Context("if a subscription_id is provided", func() {
			It("connects to doppler servers with correct parameters", func() {
				req, _ := http.NewRequest("GET", "/firehose/abc-123", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Eventually(channelGroupConnector.getPath).Should(Equal("firehose"))
				Eventually(channelGroupConnector.getStreamId).Should(Equal("abc-123"))
				Eventually(channelGroupConnector.getReconnect).Should(BeTrue())
			})

			It("returns an unauthorized status and sets the WWW-Authenticate header if authorization fails", func() {
				adminAuth.Result = AuthorizerResult{Authorized: false, ErrorMessage: "Error: Invalid authorization"}

				req, _ := http.NewRequest("GET", "/firehose/abc-123", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)

				Expect(adminAuth.TokenParam).To(Equal("token"))

				Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
				Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
				Expect(recorder.Body.String()).To(Equal("You are not authorized. Error: Invalid authorization"))
			})
		})

		Context("if subscription_id is not provided (no trailing slash)", func() {
			It("returns a 404", func() {
				req, _ := http.NewRequest("GET", "/firehose", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
				Expect(recorder.Body.String()).To(Equal("Firehose SUBSCRIPTION_ID missing. Make request to /firehose/SUBSCRIPTION_ID"))
			})

			It("It does not attempt to connect to doppler", func() {
				req, _ := http.NewRequest("GET", "/firehose", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Consistently(channelGroupConnector.getPath).Should(Equal(""))
				Consistently(channelGroupConnector.getStreamId).Should(Equal(""))
				Consistently(channelGroupConnector.getReconnect).Should(BeFalse())
			})
		})

		Context("if subscription_id is not provided (with trailing slash)", func() {
			It("returns a 404", func() {
				req, _ := http.NewRequest("GET", "/firehose/", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
				Expect(recorder.Body.String()).To(Equal("Firehose SUBSCRIPTION_ID missing. Make request to /firehose/SUBSCRIPTION_ID"))
			})

			It("It does not attempt to connect to doppler", func() {
				req, _ := http.NewRequest("GET", "/firehose/", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Consistently(channelGroupConnector.getPath).Should(Equal(""))
				Consistently(channelGroupConnector.getStreamId).Should(Equal(""))
				Consistently(channelGroupConnector.getReconnect).Should(BeFalse())
			})
		})

		Context("if there is an extra trailing slash after a valid firehose URL", func() {
			It("returns a 404", func() {
				req, _ := http.NewRequest("GET", "/firehose/abc/123", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
				Expect(recorder.Body.String()).To(Equal("Firehose SUBSCRIPTION_ID missing. Make request to /firehose/SUBSCRIPTION_ID"))
			})

			It("It does not attempt to connect to doppler", func() {
				req, _ := http.NewRequest("GET", "/firehose/abc/123", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Consistently(channelGroupConnector.getPath).Should(Equal(""))
				Consistently(channelGroupConnector.getStreamId).Should(Equal(""))
				Consistently(channelGroupConnector.getReconnect).Should(BeFalse())
			})
		})

		Context("if path is invalid but contains the string 'firehose'", func() {
			It("returns a 404", func() {
				req, _ := http.NewRequest("GET", "/appsfirehosefoo/abc-123", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})

			It("It does not attempt to connect to doppler", func() {
				req, _ := http.NewRequest("GET", "/appsfirehosefoo/abc-123", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Consistently(channelGroupConnector.getPath).Should(Equal(""))
				Consistently(channelGroupConnector.getStreamId).Should(Equal(""))
				Consistently(channelGroupConnector.getReconnect).Should(BeFalse())
			})
		})

		Context("if attempting to access the 'firehose' app", func() {
			It("It does not connect to the endpoint of type 'firehose' in doppler", func() {
				req, _ := http.NewRequest("GET", "/apps/firehose/stream", nil)
				req.Header.Add("Authorization", "token")

				proxy.ServeHTTP(recorder, req)
				Consistently(channelGroupConnector.getPath).ShouldNot(Equal("firehose"))
				Eventually(channelGroupConnector.getStreamId).Should(Equal("firehose"))
				Eventually(channelGroupConnector.getReconnect).Should(BeTrue())
			})
		})
	})

	Context("Other invalid paths", func() {
		It("returns a 404 for an empty path", func() {
			req, _ := http.NewRequest("GET", "/", nil)
			proxy.ServeHTTP(recorder, req)
			Expect(recorder.Code).To(Equal(http.StatusNotFound))
			Expect(recorder.Body.String()).To(Equal("Resource Not Found."))
		})
	})

	It("returns a 404 if the path does not start with /apps or /firehose or /set-cookie", func() {
		req, _ := http.NewRequest("GET", "/notApps", nil)
		proxy.ServeHTTP(recorder, req)
		Expect(recorder.Code).To(Equal(http.StatusNotFound))
		Expect(recorder.Body.String()).To(Equal("Resource Not Found."))
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
