package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/plumbing"

	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StreamHandler", func() {
	var (
		auth         LogAuthorizer
		adminAuth    AdminAuthorizer
		dopplerProxy *proxy.DopplerProxy
		recorder     *httptest.ResponseRecorder

		connector  *SpyGRPCConnector
		mockSender *testhelper.SpyMetricClient
		mockHealth *mockHealth
	)

	BeforeEach(func() {
		auth = LogAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		adminAuth = AdminAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}

		connector = newSpyGRPCConnector(nil)
		mockSender = testhelper.NewMetricClient()
		mockHealth = newMockHealth()

		dopplerProxy = proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			connector,
			"cookieDomain",
			50*time.Millisecond,
			mockSender,
			mockHealth,
		)

		recorder = httptest.NewRecorder()
	})

	Context("if the app id is forbidden", func() {
		It("returns a not found status", func() {
			auth.Result = AuthorizerResult{Status: http.StatusForbidden, ErrorMessage: http.StatusText(http.StatusForbidden)}

			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("if the app id is not found", func() {
		It("returns a not found status", func() {
			auth.Result = AuthorizerResult{Status: http.StatusNotFound, ErrorMessage: http.StatusText(http.StatusNotFound)}

			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("if any other error occurs", func() {
		It("returns an Internal Server Error", func() {
			auth.Result = AuthorizerResult{Status: http.StatusInternalServerError, ErrorMessage: "some bad error"}

			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Context("if authorization fails", func() {
		It("returns an unauthorized status and sets the WWW-Authenticate header", func() {
			auth.Result = AuthorizerResult{Status: http.StatusUnauthorized, ErrorMessage: "Error: Invalid authorization"}

			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(auth.TokenParam).To(Equal("token"))
			Expect(auth.Target).To(Equal("abc123"))

			Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
			Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
		})

		It("does not attempt to connect to doppler", func() {
			auth.Result = AuthorizerResult{Status: http.StatusUnauthorized, ErrorMessage: "Authorization Failed"}

			req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
			req.Header.Add("Authorization", "token")

			dopplerProxy.ServeHTTP(recorder, req)

			Expect(connector.subscriptions).To(BeNil())
		})
	})

	It("can read the authorization information from a cookie", func() {
		auth.Result = AuthorizerResult{Status: http.StatusUnauthorized, ErrorMessage: "Authorization Failed"}

		req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)

		req.AddCookie(&http.Cookie{Name: "authorization", Value: "cookie-token"})

		dopplerProxy.ServeHTTP(recorder, req)

		Expect(auth.TokenParam).To(Equal("cookie-token"))
	})

	It("connects to doppler servers with correct parameters", func() {
		req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
		req.Header.Add("Authorization", "token")

		dopplerProxy.ServeHTTP(recorder, req)

		Expect(connector.subscriptions.request).To(Equal(
			&plumbing.SubscriptionRequest{
				Filter: &plumbing.Filter{
					AppID: "abc123",
				},
			},
		))
	})

	It("closes the context when the client closes its connection", func() {
		req, _ := http.NewRequest("GET", "/apps/abc123/stream", nil)
		req.Header.Add("Authorization", "token")

		dopplerProxy.ServeHTTP(recorder, req)

		Eventually(connector.subscriptions.ctx.Done).Should(BeClosed())
	})
})
