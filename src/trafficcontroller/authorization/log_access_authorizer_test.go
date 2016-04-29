package authorization_test

import (
	"crypto/tls"
	"trafficcontroller/authorization"

	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime/pprof"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogAccessAuthorizer", func() {

	var (
		transport *http.Transport
		logger    *gosteno.Logger = loggertesthelper.Logger()
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
		It("returns true", func() {
			authorizer := authorization.NewLogAccessAuthorizer(true, "http://cloudcontroller.example.com")
			Expect(authorizer("bearer anything", "myAppId", logger)).To(Equal(true))

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
			authorizer := authorization.NewLogAccessAuthorizer(false, server.URL)

			authorized, err := authorizer("", "myAppId", logger)
			Expect(authorized).To(Equal(false))
			Expect(err).To(Equal(errors.New(authorization.NO_AUTH_TOKEN_PROVIDED_ERROR_MESSAGE)))
		})

		It("allows access when the api returns 200, and otherwise denies access", func() {
			authorizer := authorization.NewLogAccessAuthorizer(false, server.URL)

			authorized, err := authorizer("bearer something", "myAppId", logger)
			Expect(authorized).To(Equal(true))
			Expect(err).To(BeNil())

			authorized, err = authorizer("bearer something", "notMyAppId", logger)
			Expect(authorized).To(Equal(false))
			Expect(err).To(Equal(errors.New(authorization.INVALID_AUTH_TOKEN_ERROR_MESSAGE)))

			authorized, err = authorizer("bearer something", "nonExistantAppId", logger)
			Expect(authorized).To(Equal(false))
			Expect(err).To(Equal(errors.New(authorization.INVALID_AUTH_TOKEN_ERROR_MESSAGE)))
		})

		It("has no leaking go routines", func() {
			authorizer := authorization.NewLogAccessAuthorizer(false, server.URL)
			authorizer("bearer something", "myAppId", logger)

			otherGoRoutines := func() bool {
				var buf bytes.Buffer
				goRoutineProfiles := pprof.Lookup("goroutine")
				goRoutineProfiles.WriteTo(&buf, 2)

				match, err := regexp.Match("readLoop", buf.Bytes())
				Expect(err).To(BeNil(), "Unable to match /readLoop/ regexp against goRoutineProfile")
				if match {
					return match
				}

				match, err = regexp.Match("writeLoop", buf.Bytes())
				Expect(err).To(BeNil(), "Unable to match /writeLoop/ regexp against goRoutineProfile")

				return match
			}

			Eventually(otherGoRoutines).Should(Equal(false))
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
			authorizer := authorization.NewLogAccessAuthorizer(false, server.URL)
			authorized, err := authorizer("bearer something", "myAppId", logger)
			Expect(authorized).To(Equal(true))
			Expect(err).To(BeNil())
		})

		It("does not allow access when cert verifcation is not skipped", func() {
			transport.TLSClientConfig.InsecureSkipVerify = false
			authorizer := authorization.NewLogAccessAuthorizer(false, server.URL)
			authorized, err := authorizer("bearer something", "myAppId", logger)
			Expect(authorized).To(Equal(false))
			Expect(err).To(Equal(errors.New(authorization.INVALID_AUTH_TOKEN_ERROR_MESSAGE)))
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
