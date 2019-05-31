package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("DopplerProxy", func() {
	var (
		auth         LogAuthorizer
		adminAuth    AdminAuthorizer
		dopplerProxy *proxy.DopplerProxy
		recorder     *httptest.ResponseRecorder

		mockGrpcConnector       *mockGrpcConnector
		mockDopplerStreamClient *mockReceiver
		mockHealth              *mockHealth

		mockSender *testhelper.SpyMetricClient

		recentLogsHandler *spyRecentLogsHandler
		logCacheClient    *fakeLogCacheClient
	)

	BeforeEach(func() {
		auth = LogAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		adminAuth = AdminAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}

		mockGrpcConnector = newMockGrpcConnector()
		mockDopplerStreamClient = newMockReceiver()
		mockSender = testhelper.NewMetricClient()
		mockHealth = newMockHealth()

		mockGrpcConnector.SubscribeOutput.Ret0 <- mockDopplerStreamClient.Recv

		recentLogsHandler = newSpyRecentLogsHandler()

		logCacheClient = newFakeLogCacheClient()

		dopplerProxy = proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			mockGrpcConnector,
			"cookieDomain",
			50*time.Millisecond,
			time.Hour,
			mockSender,
			mockHealth,
			recentLogsHandler,
			false,
			logCacheClient,
		)

		recorder = httptest.NewRecorder()
	})

	JustBeforeEach(func() {
		close(mockGrpcConnector.SubscribeOutput.Ret0)
		close(mockGrpcConnector.SubscribeOutput.Ret1)

		mockDopplerStreamClient.RecvOutput.Ret0 <- []byte("test-message")
		close(mockDopplerStreamClient.RecvOutput.Ret0)
		close(mockDopplerStreamClient.RecvOutput.Ret1)
	})

	Describe("metrics", func() {
		It("emits latency value metric for recentlogs request", func() {
			req, _ := http.NewRequest("GET", "/apps/12bdb5e8-ba61-48e3-9dda-30ecd1446663/recentlogs", nil)
			metricName := "doppler_proxy.recent_logs_latency"
			requestStart := time.Now()

			dopplerProxy.ServeHTTP(recorder, req)

			metricValue := mockSender.GetValue(metricName)
			elapsed := float64(time.Since(requestStart)) / float64(time.Millisecond)
			Expect(metricValue).To(BeNumerically("<", elapsed))
		})

		DescribeTable("increments a counter for every envelope that is written", func(url, endpoint string) {
			server := httptest.NewServer(dopplerProxy)
			defer server.CloseClientConnections()

			_, _, err := websocket.DefaultDialer.Dial(
				wsEndpoint(server, url),
				http.Header{"Authorization": []string{"token"}},
			)
			Expect(err).ToNot(HaveOccurred())

			f := func() uint64 {
				for _, e := range mockSender.GetEnvelopes("egress") {
					t, ok := e.DeprecatedTags["endpoint"]
					if !ok {
						continue
					}

					if t.GetText() != endpoint {
						continue
					}

					return e.GetCounter().GetDelta()
				}

				return 0
			}
			Eventually(f).Should(Equal(uint64(1)))
		},
			Entry("stream requests", "/apps/12bdb5e8-ba61-48e3-9dda-30ecd1446663/stream", "stream"),
			Entry("firehose requests", "/firehose/streamID", "firehose"),
		)

		It("sets the health value for firehose count", func() {
			req, _ := http.NewRequest("GET", "/firehose/streamID", nil)
			dopplerProxy.ServeHTTP(recorder, req)
			Eventually(mockHealth.SetInput.Name, 3).Should(Receive(Equal("firehoseStreamCount")))
		})

		It("sets the health value for app stream count", func() {
			req, _ := http.NewRequest("GET", "/apps/appID/stream", nil)
			dopplerProxy.ServeHTTP(recorder, req)
			Eventually(mockHealth.SetInput.Name, 3).Should(Receive(Equal("appStreamCount")))
		})
	})

	It("calls the recent logs handler", func() {
		req, _ := http.NewRequest("GET", "/apps/8de7d390-9044-41ff-ab76-432299923511/recentlogs", nil)
		req.Header.Add("Authorization", "token")

		dopplerProxy.ServeHTTP(recorder, req)

		Eventually(func() int { return recentLogsHandler.numCalls }).Should(Equal(1))
	})

	It("rejects badly-formed app GUIDs", func() {
		req, _ := http.NewRequest("GET", "/apps/not-a-valid-guid/recentlogs?limit=2", nil)
		req.Header.Add("Authorization", "token")

		dopplerProxy.ServeHTTP(recorder, req)
		Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
	})

	It("accepts non-GUIDs when disableAccessControl is set", func() {
		req, _ := http.NewRequest("GET", "/apps/not-a-valid-guid/recentlogs?limit=2", nil)
		req.Header.Add("Authorization", "token")

		dopplerProxy = proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			mockGrpcConnector,
			"cookieDomain",
			50*time.Millisecond,
			time.Hour,
			mockSender,
			mockHealth,
			recentLogsHandler,
			true,
			logCacheClient,
		)
		dopplerProxy.ServeHTTP(recorder, req)
		Expect(recorder.Code).To(Equal(http.StatusOK))
	})

	Context("SetCookie", func() {
		It("returns an OK status with a form", func() {
			req, _ := http.NewRequest("POST", "/set-cookie", strings.NewReader("CookieName=cookie&CookieValue=monster"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(recorder.Header().Get("Set-Cookie")).To(Equal("cookie=monster; Domain=cookieDomain; HttpOnly; Secure"))
		})

		It("returns a bad request if the form does not parse", func() {
			req, _ := http.NewRequest("POST", "/set-cookie", nil)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("CORS requests", func() {
		It("configures CORS for set-cookie", func() {
			req, _ := http.NewRequest(
				"POST",
				"/set-cookie",
				strings.NewReader("CookieName=cookie&CookieValue=monster"),
			)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", "fake-origin-string")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(recorder.Header().Get("Access-Control-Allow-Origin")).To(Equal("fake-origin-string"))
			Expect(recorder.Header().Get("Access-Control-Allow-Credentials")).To(Equal("true"))
			Expect(recorder.Header().Get("Access-Control-Allow-Headers")).To(Equal("Content-Type"))
		})

		It("configures CORS for recentlogs", func() {
			req, _ := http.NewRequest(
				"GET",
				"/apps/guid/recentlogs",
				nil,
			)
			req.Header.Set("Origin", "fake-origin-string")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(recorder.Header().Get("Access-Control-Allow-Origin")).To(Equal("fake-origin-string"))
			Expect(recorder.Header().Get("Access-Control-Allow-Credentials")).To(Equal("true"))
			Expect(recorder.Header().Get("Access-Control-Allow-Headers")).To(Equal(""))
		})
	})
})

type spyRecentLogsHandler struct {
	numCalls int
}

func (rlh *spyRecentLogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rlh.numCalls += 1
}

func newSpyRecentLogsHandler() *spyRecentLogsHandler {
	return &spyRecentLogsHandler{}
}
