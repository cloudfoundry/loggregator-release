package middleware_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"trafficcontroller/middleware"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
)

var _ = Describe("AccessHandler", func() {
	const (
		host = "1.2.3.4"
		port = 1234
	)
	var (
		accessHandler    *middleware.AccessHandler
		mockHandler      *mockHttpHandler
		mockAccessLogger *mockAccessLogger
	)

	BeforeEach(func() {
		gostLogger := loggertesthelper.Logger()
		mockHandler = newMockHttpHandler()
		mockAccessLogger = newMockAccessLogger()
		accessMiddleware := middleware.Access(mockAccessLogger, host, port, gostLogger)
		accessHandler = accessMiddleware(mockHandler)

		var _ http.Handler = accessHandler
	})

	Describe("ServeHTTP", func() {
		It("Logs the access", func() {
			mockAccessLogger.LogAccessOutput.Ret0 <- nil
			req, err := newServerRequest("GET", "https://foo.bar/baz", nil)
			Expect(err).ToNot(HaveOccurred())
			resp := httptest.NewRecorder()
			accessHandler.ServeHTTP(resp, req)

			Eventually(mockHandler.ServeHTTPInput).Should(BeCalled(With(resp, req)))
			Expect(mockAccessLogger.LogAccessCalled).To(HaveLen(1))
			Expect(mockAccessLogger.LogAccessInput.Req).To(BeCalled(With(req)))
			Expect(mockAccessLogger.LogAccessInput.Host).To(BeCalled(With(host)))
			Expect(mockAccessLogger.LogAccessInput.Port).To(BeCalled(With(uint32(port))))
		})
	})
})

func newServerRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	req.Host = req.URL.Host
	if req.URL.Scheme == "https" {
		req.TLS = &tls.ConnectionState{}
	}
	req.URL = &url.URL{
		Path:     req.URL.Path,
		RawQuery: req.URL.RawQuery,
	}
	return req, nil
}
