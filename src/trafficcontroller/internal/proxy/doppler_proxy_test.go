package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"code.cloudfoundry.org/loggregator-release/src/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator-release/src/trafficcontroller/internal/proxy"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo/v2"
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

		mockSender *testhelper.SpyMetricClient
	)

	BeforeEach(func() {
		auth = LogAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		adminAuth = AdminAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}

		mockGrpcConnector = newMockGrpcConnector()
		mockDopplerStreamClient = newMockReceiver()
		mockSender = testhelper.NewMetricClient()

		mockGrpcConnector.SubscribeOutput.Ret0 <- mockDopplerStreamClient.Recv

		dopplerProxy = proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			mockGrpcConnector,
			"cookieDomain",
			time.Hour,
			mockSender,
			false,
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
	})
})
