package main_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	app "tools/smokes/http_drain"
)

func TestHttpDrain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HttpDrain Suite")
}

var _ = Describe("HTTP Drain", func() {
	payload := []byte(`<34>1 2003-10-11T22:14:15.003Z mymachine.example.com su 12345 98765 [exampleSDID@32473 iut="3" eventSource="Application" eventID="1011"] 'su root' failed for lonvick on /dev/pts/8`)

	Context("the syslog endpoint", func() {
		It("increments a counter for valid syslog messages", func() {
			handler := app.NewSyslog()
			server := httptest.NewServer(handler)

			resp, err := http.Post(server.URL+"/drain", "blah", bytes.NewReader(payload))
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			resp, err = http.Get(server.URL + "/count")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(BeEquivalentTo("1"))
		})

		It("does nothing with an invalid message", func() {
			handler := app.NewSyslog()
			server := httptest.NewServer(handler)

			resp, err := http.Post(server.URL+"/drain", "blah", nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

			resp, err = http.Get(server.URL + "/count")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(BeEquivalentTo("0"))
		})
	})

})
