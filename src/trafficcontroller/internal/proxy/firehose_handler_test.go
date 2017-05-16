package proxy_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"plumbing"
	"trafficcontroller/internal/proxy"

	"golang.org/x/net/context"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FirehoseHandler", func() {
	var (
		handler http.Handler

		auth      LogAuthorizer
		adminAuth AdminAuthorizer
		recorder  *httptest.ResponseRecorder
		connector *SpyGRPCConnector
	)

	BeforeEach(func() {
		connector = newSpyGRPCConnector(nil)

		adminAuth = AdminAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}
		auth = LogAuthorizer{Result: AuthorizerResult{Status: http.StatusOK}}

		recorder = httptest.NewRecorder()

		handler = proxy.NewDopplerProxy(
			auth.Authorize,
			adminAuth.Authorize,
			connector,
			"cookieDomain",
			50*time.Millisecond,
		)
	})

	Context("if a subscription_id is provided", func() {
		It("connects to doppler servers with correct parameters", func() {
			req, _ := http.NewRequest("GET", "/firehose/abc-123", nil)
			req.Header.Add("Authorization", "token")

			handler.ServeHTTP(recorder, req.WithContext(
				context.WithValue(req.Context(), "subID", "abc-123")),
			)

			var request subscribeRequest
			Eventually(connector.subscriptions).Should(Receive(&request))
			Expect(request.request).To(Equal(&plumbing.SubscriptionRequest{
				ShardID: "abc-123",
			}))
		})

		It("returns an unauthorized status and sets the WWW-Authenticate header if authorization fails", func() {
			adminAuth.Result = AuthorizerResult{Status: http.StatusUnauthorized, ErrorMessage: "Error: Invalid authorization"}

			req, _ := http.NewRequest("GET", "/firehose/abc-123", nil)
			req.Header.Add("Authorization", "token")

			handler.ServeHTTP(recorder, req)

			Expect(adminAuth.TokenParam).To(Equal("token"))

			Expect(recorder.Code).To(Equal(http.StatusUnauthorized))
			Expect(recorder.HeaderMap.Get("WWW-Authenticate")).To(Equal("Basic"))
			Expect(recorder.Body.String()).To(Equal("You are not authorized. Error: Invalid authorization"))
		})
	})

	It("returns a 404 if subscription_id is not provided", func() {
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
		)
		server := httptest.NewServer(handler)
		defer server.CloseClientConnections()

		conn, _, err := websocket.DefaultDialer.Dial(
			wsEndpoint(server, "/firehose/subscription-id"),
			http.Header{"Authorization": []string{"token"}},
		)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()

		f := func() fake.Metric {
			return fakeMetricSender.GetValue("dopplerProxy.firehoses")
		}
		Eventually(f, 4).Should(Equal(fake.Metric{Value: 1, Unit: "connections"}))
	})
})

func wsEndpoint(server *httptest.Server, path string) string {
	return strings.Replace(server.URL, "http", "ws", 1) + path
}
