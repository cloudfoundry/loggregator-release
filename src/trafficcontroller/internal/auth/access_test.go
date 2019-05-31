package auth_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/auth"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AccessHandler", func() {
	const (
		host = "1.2.3.4"
		port = 1234
	)
	var (
		accessHandler *auth.AccessHandler
		handler       *spyHandler
		spyLogger     *spyAccessLogger
	)

	BeforeEach(func() {
		handler = &spyHandler{}
		spyLogger = &spyAccessLogger{}
		accessMiddleware := auth.Access(spyLogger, host, port)
		accessHandler = accessMiddleware(handler)

		var _ http.Handler = accessHandler
	})

	Describe("ServeHTTP", func() {
		It("Logs the access", func() {
			req, err := newServerRequest("GET", "https://foo.bar/baz", nil)
			Expect(err).ToNot(HaveOccurred())
			resp := httptest.NewRecorder()
			accessHandler.ServeHTTP(resp, req)

			Expect(handler.response).To(Equal(resp))
			Expect(handler.request).To(Equal(req))
			Expect(spyLogger.request).To(Equal(req))
			Expect(spyLogger.host).To(Equal(host))
			Expect(spyLogger.port).To(Equal(uint32(port)))
		})
	})
})

type spyHandler struct {
	response http.ResponseWriter
	request  *http.Request
}

func (s *spyHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	s.response = resp
	s.request = req
}

type spyAccessLogger struct {
	request *http.Request
	host    string
	port    uint32
}

func (s *spyAccessLogger) LogAccess(req *http.Request, host string, port uint32) error {
	s.request = req
	s.host = host
	s.port = port
	return nil
}
