package accesslogger_test

import (
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"trafficcontroller/accesslogger"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Accesslogger", func() {
	var (
		writer *mockWriter
		logger *accesslogger.AccessLogger
	)

	BeforeEach(func() {
		writer = newMockWriter()
		logger = accesslogger.New(writer)
	})

	Describe("LogAccess", func() {
		Context("Happy", func() {
			BeforeEach(func() {
				writer.WriteOutput.Err <- nil
				writer.WriteOutput.Sent <- 0
			})

			It("Logs Access", func() {
				req, err := newServerRequest("GET", "some.url.com/foo", nil)
				Expect(err).ToNot(HaveOccurred())

				Expect(logger.LogAccess(req)).To(Succeed())

				Expect(writer.WriteInput.Message).To(Receive())
			})

			It("Includes details about the access", func() {
				req, err := newServerRequest("GET", "http://some.url.com/foo", nil)
				Expect(err).ToNot(HaveOccurred())
				req.RemoteAddr = "127.0.0.1:4567"

				Expect(logger.LogAccess(req)).To(Succeed())

				expected := "127.0.0.1:4567 GET: http://some.url.com/foo\n"
				Expect(writer.WriteInput.Message).To(Receive(BeEquivalentTo(expected)))
			})

			It("Uses X-Forwarded-For if it exists", func() {
				req, err := newServerRequest("GET", "http://some.url.com/foo", nil)
				Expect(err).ToNot(HaveOccurred())
				req.RemoteAddr = "127.0.0.1:4567"
				req.Header.Set("X-Forwarded-For", "50.60.70.80:1234")

				Expect(logger.LogAccess(req)).To(Succeed())

				expected := "50.60.70.80:1234 GET: http://some.url.com/foo\n"
				Expect(writer.WriteInput.Message).To(Receive(BeEquivalentTo(expected)))
			})

			It("Writes multiple log lines", func() {
				req, err := newServerRequest("GET", "http://some.url.com/foo", nil)
				Expect(err).ToNot(HaveOccurred())
				req.RemoteAddr = "127.0.0.1:4567"

				Expect(logger.LogAccess(req)).To(Succeed())

				writer.WriteOutput.Err <- nil
				writer.WriteOutput.Sent <- 0
				req.Header.Set("X-Forwarded-For", "50.60.70.80:1234")
				Expect(logger.LogAccess(req)).To(Succeed())

				first := "127.0.0.1:4567 GET: http://some.url.com/foo\n"
				second := "50.60.70.80:1234 GET: http://some.url.com/foo\n"
				Expect(writer.WriteInput.Message).To(BeEquivalentToEach(
					first,
					second,
				))
			})
		})

		Context("Unhappy", func() {
			BeforeEach(func() {
				writer.WriteOutput.Err <- errors.New("boom")
				writer.WriteOutput.Sent <- 0
			})

			It("Errors", func() {
				req, err := newServerRequest("GET", "http://some.url.com/foo", nil)
				Expect(err).ToNot(HaveOccurred())
				req.RemoteAddr = "127.0.0.1:4567"
				Expect(logger.LogAccess(req)).ToNot(Succeed())
			})
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
