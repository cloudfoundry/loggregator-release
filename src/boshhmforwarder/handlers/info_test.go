package handlers_test

import (
	"boshhmforwarder/handlers"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"

	"encoding/json"

	. "github.com/onsi/gomega"
)

var _ = Describe("InfoHandler", func() {

	var (
		req      *http.Request
		recorder *httptest.ResponseRecorder
		handler  http.Handler
	)

	BeforeEach(func() {
		recorder = httptest.NewRecorder()
		handler = handlers.NewInfoHandler()
		req = createRequest("GET", "/info")
	})

	Context("with a good version file", func() {
		BeforeEach(func() {

		})

		It("returns the current time (in ms) and version", func() {
			handler.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(http.StatusOK))

			var bodyContents map[string]string

			Expect(json.Unmarshal(recorder.Body.Bytes(), &bodyContents)).To(Succeed())

			upTime, err := time.ParseDuration(bodyContents["uptime"])
			Expect(err).ToNot(HaveOccurred())

			Expect(upTime.Seconds()).To(BeNumerically("<", 1))
			Expect(upTime.Seconds()).To(BeNumerically(">", 0))
		})
	})
})

func createRequest(method, url string) *http.Request {
	req, err := http.NewRequest(method, url, nil)
	Expect(err).ToNot(HaveOccurred())

	return req
}
