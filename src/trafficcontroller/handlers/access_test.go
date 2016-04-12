package handlers_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"trafficcontroller/handlers"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
)

var _ = Describe("Handler", func() {
	var (
		handler          *handlers.AccessHandler
		mockHandler      *mockHttpHandler
		mockAccessLogger *mockAccessLogger
	)

	BeforeEach(func() {
		gostLogger := loggertesthelper.Logger()
		mockHandler = newMockHttpHandler()
		mockAccessLogger = newMockAccessLogger()
		handler = handlers.NewAccess(mockHandler, mockAccessLogger, gostLogger)
		var _ http.Handler = handler
	})

	Describe("ServeHTTP", func() {
		It("Logs the access", func() {
			mockAccessLogger.LogAccessOutput.Ret0 <- nil
			req, err := newServerRequest("GET", "https://foo.bar/baz", nil)
			Expect(err).ToNot(HaveOccurred())
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)

			Eventually(mockHandler.ServeHTTPInput).Should(BeCalled(With(resp, req)))
			Expect(mockAccessLogger.LogAccessCalled).To(HaveLen(1))
			Expect(mockAccessLogger.LogAccessInput.Req).To(BeCalled(With(req)))
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
