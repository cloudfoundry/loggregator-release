package proxy_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/plumbing"

	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"

	"golang.org/x/net/context"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FirehoseHandler", func() {
	var (
		auth       LogAuthorizer
		adminAuth  AdminAuthorizer
		recorder   *httptest.ResponseRecorder
		connector  *SpyGRPCConnector
		mockSender *testhelper.SpyMetricClient
		mockHealth *mockHealth
	)

	BeforeEach(func() {
		connector = newSpyGRPCConnector(nil)

		adminAuth = AdminAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		auth = LogAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}

		recorder = httptest.NewRecorder()
		mockSender = testhelper.NewMetricClient()
		mockHealth = newMockHealth()
	})

	It("connects to doppler servers with correct parameters", func() {
		handler := proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			connector,
			"cookieDomain",
			50*time.Millisecond,
			mockSender,
			mockHealth,
		)

		req, _ := http.NewRequest("GET", "/firehose/abc-123", nil)
		req.Header.Add("Authorization", "token")

		handler.ServeHTTP(recorder, req.WithContext(
			context.WithValue(req.Context(), "subID", "abc-123")),
		)

		Expect(connector.subscriptions.request).To(Equal(&plumbing.SubscriptionRequest{
			ShardID: "abc-123",
		}))
	})

	It("accepts a query param for filtering logs", func() {
		_ = proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			connector,
			"cookieDomain",
			50*time.Millisecond,
			mockSender,
			mockHealth,
		)

		req, err := http.NewRequest("GET", "/firehose/123?filter-type=logs", nil)
		Expect(err).NotTo(HaveOccurred())

		h := proxy.NewFirehoseHandler(connector, proxy.NewWebSocketServer(
			testhelper.NewMetricClient(),
		), mockSender)
		h.ServeHTTP(recorder, req)

		Expect(connector.subscriptions.request.Filter).To(Equal(&plumbing.Filter{
			Message: &plumbing.Filter_Log{
				Log: &plumbing.LogFilter{},
			},
		}))
	})

	It("accepts a query param for filtering metrics", func() {
		_ = proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			connector,
			"cookieDomain",
			50*time.Millisecond,
			mockSender,
			mockHealth,
		)

		req, err := http.NewRequest("GET", "/firehose/123?filter-type=metrics", nil)
		Expect(err).NotTo(HaveOccurred())

		h := proxy.NewFirehoseHandler(connector, proxy.NewWebSocketServer(
			testhelper.NewMetricClient(),
		), mockSender)
		h.ServeHTTP(recorder, req)

		Expect(connector.subscriptions.request.Filter).To(Equal(&plumbing.Filter{
			Message: &plumbing.Filter_Metric{
				Metric: &plumbing.MetricFilter{},
			},
		}))
	})

	It("returns an unauthorized status and sets the WWW-Authenticate header if authorization fails", func() {
		handler := proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			connector,
			"cookieDomain",
			50*time.Millisecond,
			mockSender,
			mockHealth,
		)

		adminAuth.Result = AuthorizerResult{Status: http.StatusUnauthorized, ErrorMessage: "Error: Invalid authorization"}

		req, _ := http.NewRequest("GET", "/firehose/abc-123", nil)
		req.Header.Add("Authorization", "token")

		handler.ServeHTTP(recorder, req)

		Expect(adminAuth.TokenParam).To(Equal("token"))

		Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
		Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
		Expect(recorder.Body.String()).To(Equal("You are not authorized. Error: Invalid authorization"))
	})

	It("returns a 404 if subscription_id is not provided", func() {
		handler := proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			connector,
			"cookieDomain",
			50*time.Millisecond,
			mockSender,
			mockHealth,
		)

		req, _ := http.NewRequest("GET", "/firehose/", nil)
		req.Header.Add("Authorization", "token")

		handler.ServeHTTP(recorder, req)
		Expect(recorder.Code).To(Equal(http.StatusNotFound))
	})

	It("closes the connection to the client on error", func() {
		connector := newSpyGRPCConnector(errors.New("subscribe failed"))
		handler := proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			connector,
			"cookieDomain",
			50*time.Millisecond,
			mockSender,
			mockHealth,
		)
		server := httptest.NewServer(handler)
		defer server.CloseClientConnections()

		conn, _, err := websocket.DefaultDialer.Dial(
			wsEndpoint(server, "/firehose/subscription-id"),
			http.Header{"Authorization": []string{"token"}},
		)
		Expect(err).ToNot(HaveOccurred())

		f := func() string {
			_, _, err := conn.ReadMessage()
			return fmt.Sprintf("%s", err)
		}
		Eventually(f).Should(ContainSubstring("websocket: close 1000"))
	})

	It("emits the number of connections as a metric", func() {
		connector := newSpyGRPCConnector(nil)
		handler := proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			connector,
			"cookieDomain",
			50*time.Millisecond,
			mockSender,
			mockHealth,
		)
		server := httptest.NewServer(handler)
		defer server.CloseClientConnections()

		conn, _, err := websocket.DefaultDialer.Dial(
			wsEndpoint(server, "/firehose/subscription-id"),
			http.Header{"Authorization": []string{"token"}},
		)
		Expect(err).ToNot(HaveOccurred())

		f := func() float64 {
			return mockSender.GetValue("doppler_proxy.firehoses")
		}
		Eventually(f, 1, "100ms").Should(Equal(1.0))
		conn.Close()
	})
})

func wsEndpoint(server *httptest.Server, path string) string {
	return strings.Replace(server.URL, "http", "ws", 1) + path
}
