package auth_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/auth"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AccessHandler", func() {
	const (
		host = "1.2.3.4"
		port = 1234
	)
	var (
		accessHandler    *auth.AccessHandler
		mockHandler      *mockHttpHandler
		mockAccessLogger *mockAccessLogger
	)

	BeforeEach(func() {
		mockHandler = newMockHttpHandler()
		mockAccessLogger = newMockAccessLogger()
		accessMiddleware := auth.Access(mockAccessLogger, host, port)
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
