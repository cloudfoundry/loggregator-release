package auth_test

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"

	"trafficcontroller/internal/auth"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogAccessAuthorizer", func() {

	var (
		transport *http.Transport
		server    *httptest.Server
	)

	BeforeEach(func() {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		http.DefaultClient.Transport = transport
	})

	Context("Disable Access Control", func() {
		It("returns http.StatusOK", func() {
			authorizer := auth.NewLogAccessAuthorizer(true, "http://cloudcontroller.example.com")
			Expect(authorizer("bearer anything", "myAppId")).To(Equal(http.StatusOK))
		})
	})

	Context("Server does not use SSL", func() {

		BeforeEach(func() {
			server = startHTTPServer()
		})

		AfterEach(func() {
			server.Close()
		})

		It("does not allow access for requests with empty AuthTokens", func() {
			authorizer := auth.NewLogAccessAuthorizer(false, server.URL)

			status, err := authorizer("", "myAppId")
			Expect(status).To(Equal(http.StatusUnauthorized))
			Expect(err).To(Equal(errors.New(auth.NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)))
		})

		It("allows access when the api returns 200, and otherwise denies access", func() {
			authorizer := auth.NewLogAccessAuthorizer(false, server.URL)

			status, err := authorizer("bearer something", "myAppId")
			Expect(status).To(Equal(http.StatusOK))
			Expect(err).To(BeNil())

			status, err = authorizer("bearer something", "notMyAppId")
			Expect(status).To(Equal(http.StatusForbidden))
			Expect(err).To(MatchError(http.StatusText(http.StatusForbidden)))

			status, err = authorizer("bearer something", "nonExistantAppId")
			Expect(status).To(Equal(http.StatusNotFound))
			Expect(err).To(MatchError(http.StatusText(http.StatusNotFound)))
		})
	})

	Context("Server uses SSL without valid certificate", func() {
		BeforeEach(func() {
			server = startHTTPSServer()
		})

		AfterEach(func() {
			server.Close()
		})

		It("does allow access when cert verification is skipped", func() {
			authorizer := auth.NewLogAccessAuthorizer(false, server.URL)
			status, err := authorizer("bearer something", "myAppId")
			Expect(status).To(Equal(http.StatusOK))
			Expect(err).To(BeNil())
		})

		It("does not allow access when cert verifcation is not skipped", func() {
			transport.TLSClientConfig.InsecureSkipVerify = false
			authorizer := auth.NewLogAccessAuthorizer(false, server.URL)
			status, err := authorizer("bearer something", "myAppId")
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(err).To(BeAssignableToTypeOf(&url.Error{}))
			urlErr := err.(*url.Error)
			Expect(urlErr.Err).To(MatchError("x509: certificate signed by unknown authority"))
		})
	})

})

type handler struct{}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	re := regexp.MustCompile("^/internal/log_access/([^/?]+)$")
	result := re.FindStringSubmatch(r.URL.Path)
	if len(result) != 2 {
		w.WriteHeader(500)
		return
	}

	switch result[1] {
	case "myAppId":
		w.Write([]byte("{}"))
	case "notMyAppId":
		w.WriteHeader(403)
	default:
		w.WriteHeader(404)
	}
}

func startHTTPServer() *httptest.Server {
	return httptest.NewServer(new(handler))
}

func startHTTPSServer() *httptest.Server {
	return httptest.NewTLSServer(new(handler))
}
