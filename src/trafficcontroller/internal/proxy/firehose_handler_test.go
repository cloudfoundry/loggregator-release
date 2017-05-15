package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	"plumbing"
	"trafficcontroller/internal/proxy"

	"golang.org/x/net/context"

	. "github.com/apoydence/eachers"
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
		connector = newSpyGRPCConnector()

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

			expectedRequest := &plumbing.SubscriptionRequest{
				ShardID: "abc-123",
			}
			Eventually(connector.subscriptionRequests).Should(BeCalled(With(expectedRequest)))
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

	Context("if subscription_id is not provided", func() {
		It("returns a 404", func() {
			req, _ := http.NewRequest("GET", "/firehose/", nil)
			req.Header.Add("Authorization", "token")

			handler.ServeHTTP(recorder, req)
			Expect(recorder.Code).To(Equal(http.StatusNotFound))
		})
	})
})

type SpyGRPCConnector struct {
	subscriptionRequests chan *plumbing.SubscriptionRequest
}

func newSpyGRPCConnector() *SpyGRPCConnector {
	return &SpyGRPCConnector{
		subscriptionRequests: make(chan *plumbing.SubscriptionRequest, 100),
	}
}

func (s *SpyGRPCConnector) Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (func() ([]byte, error), error) {
	s.subscriptionRequests <- req

	return func() ([]byte, error) { return []byte("a-slice"), nil }, nil
}
func (s *SpyGRPCConnector) ContainerMetrics(ctx context.Context, appID string) [][]byte {
	return nil
}
func (s *SpyGRPCConnector) RecentLogs(ctx context.Context, appID string) [][]byte {
	return nil
}
