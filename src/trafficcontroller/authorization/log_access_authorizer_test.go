package authorization_test

import (
	"trafficcontroller/authorization"

	"bytes"
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
		logger *gosteno.Logger = loggertesthelper.Logger()
		server *httptest.Server
	)

	Context("Allow all access", func() {
		authorizer := authorization.NewLogAccessAuthorizer(true, "http://cloudcontroller.example.com", true)
		Expect(authorizer("bearer anything", "myAppId", logger)).To(Equal(true))
	})

	Context("Server does not use SSL", func() {

		BeforeEach(func() {
			server = startHTTPServer()
		})

		AfterEach(func() {
			server.Close()
		})

		It("allows access when the api returns 200", func() {
			authorizer := authorization.NewLogAccessAuthorizer(false, server.URL, true)

			Expect(authorizer("bearer something", "myAppId", logger)).To(Equal(true))
			Expect(authorizer("bearer something", "notMyAppId", logger)).To(Equal(false))
			Expect(authorizer("bearer something", "nonExistantAppId", logger)).To(Equal(false))

		})

		It("has no leaking go routines", func() {
			authorizer := authorization.NewLogAccessAuthorizer(false, server.URL, true)
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
			authorizer := authorization.NewLogAccessAuthorizer(false, server.URL, true)
			Expect(authorizer("bearer something", "myAppId", logger)).To(Equal(true))
		})

		It("does not allow access when cert verifcation is not skipped", func() {
			authorizer := authorization.NewLogAccessAuthorizer(false, server.URL, false)
			Expect(authorizer("bearer something", "myAppId", logger)).To(Equal(false))
		})
	})

})

type handler struct{}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	re := regexp.MustCompile("^/v2/apps/([^/?]+)$")
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
