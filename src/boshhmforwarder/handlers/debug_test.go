package handlers_test

import (
	"boshhmforwarder/handlers"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var (
	req      *http.Request
	recorder *httptest.ResponseRecorder
	handler  http.Handler
	router   *mux.Router
	err      error
)

var _ = Describe("Debug", func() {
	BeforeEach(func() {
		recorder = httptest.NewRecorder()
		handler = handlers.NewInfoHandler()
		router = mux.NewRouter()
		handlers.CreateDebugEndpoints(router, "admin", "password")
	})

	Context("with proper authorization", func() {
		DescribeTable("debug endpoints should return 200 response code", func(endpoint string) {
			req, err = http.NewRequest("GET", fmt.Sprintf("/debug/pprof%s", endpoint), nil)
			Expect(err).ToNot(HaveOccurred())

			encodedCreds := base64.StdEncoding.EncodeToString([]byte("admin:password"))
			req.Header.Add("Authorization", fmt.Sprintf("Basic %s", encodedCreds))
			router.ServeHTTP(recorder, req)
			Expect(recorder.Code).To(Equal(200))
		},
			Entry("index", ""),
			Entry("cmdline", "/cmdline"),
			Entry("profile", "/profile?seconds=1"),
			Entry("symbol", "/symbol"),
			Entry("goroutine", "/goroutine"),
			Entry("heap", "/heap"),
			Entry("threadcreate", "/threadcreate"),
			Entry("block", "/block"),
		)
	})
	Context("with bad authentication", func() {
		Context("not BasicAuth", func() {
			It("returns a 401", func() {
				req, err = http.NewRequest("GET", "/debug/pprof", nil)
				Expect(err).ToNot(HaveOccurred())

				req.Header.Add("Authorization", "Complex credentials")
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(401))
			})
		})

		Context("Bad encoding", func() {
			It("returns a 401", func() {
				req, err = http.NewRequest("GET", "/debug/pprof", nil)
				Expect(err).ToNot(HaveOccurred())

				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", "admin:password"))
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(401))
			})
		})

		Context("Malformatted credentials", func() {
			It("returns a 401", func() {
				req, err = http.NewRequest("GET", "/debug/pprof", nil)
				Expect(err).ToNot(HaveOccurred())

				encodedCreds := base64.StdEncoding.EncodeToString([]byte("admin-password"))
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", encodedCreds))
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(401))
			})
		})

		Context("Not Matching credentials", func() {
			It("returns a 401", func() {
				req, err = http.NewRequest("GET", "/debug/pprof", nil)
				Expect(err).ToNot(HaveOccurred())

				encodedCreds := base64.StdEncoding.EncodeToString([]byte("admin:wrong"))
				req.Header.Add("Authorization", fmt.Sprintf("Basic %s", encodedCreds))
				router.ServeHTTP(recorder, req)
				Expect(recorder.Code).To(Equal(401))
			})
		})
	})
})
