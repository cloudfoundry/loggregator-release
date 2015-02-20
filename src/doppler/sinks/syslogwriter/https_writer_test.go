package syslogwriter_test

import (
	"doppler/sinks/syslogwriter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"time"
)

var _ = Describe("HttpsWriter", func() {

	Context("With an HTTPS Connection", func() {

		var server *httptest.Server
		var requestChan chan []byte
		standardErrorPriority := 14

		BeforeEach(func() {
			requestChan = make(chan []byte, 1)
			server = ServeHTTP(requestChan)
		})

		AfterEach(func() {
			server.Close()
			http.DefaultServeMux = http.NewServeMux()
		})

		It("HTTP POSTs each log message to the HTTPS syslog endpoint", func() {
			outputUrl, _ := url.Parse(server.URL + "/234-bxg-234/")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true)
			err := w.Connect()
			Expect(err).ToNot(HaveOccurred())

			parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
			byteCount, err := w.Write(standardErrorPriority, []byte("Message"), "just a test", "TEST", parsedTime.UnixNano())
			Expect(byteCount).To(Equal(76))
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() string {
				return string(<-requestChan)
			}).Should(ContainSubstring("loggregator appId [just a test] - - Message"))
		})

		It("returns an error when unable to HTTP POST the log message", func() {
			outputUrl, _ := url.Parse("https://")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true)

			_, err := w.Write(standardErrorPriority, []byte("Message"), "just a test", "TEST", time.Now().UnixNano())
			Expect(err).To(HaveOccurred())
		})

		It("should close connections and return an error if status code returned is not 200", func() {
			outputUrl, _ := url.Parse(server.URL + "/doesnotexist")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true)
			err := w.Connect()
			Expect(err).ToNot(HaveOccurred())

			parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
			for i := 0; i < 10; i++ {
				_, err := w.Write(standardErrorPriority, []byte("Message"), "just a test", "TEST", parsedTime.UnixNano())
				Expect(err).To(HaveOccurred())
			}

			origNumGoRoutines := runtime.NumGoroutine()
			Eventually(runtime.NumGoroutine, 6).Should(BeNumerically("<", origNumGoRoutines))
		})

		It("should not return error for response 200 status codes", func() {
			outputUrl, _ := url.Parse(server.URL + "/234-bxg-234/")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true)
			err := w.Connect()
			Expect(err).ToNot(HaveOccurred())

			parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
			_, err = w.Write(standardErrorPriority, []byte("Message"), "just a test", "TEST", parsedTime.UnixNano())
			Expect(err).ToNot(HaveOccurred())

		})

		It("returns an error for syslog-tls scheme", func() {
			outputUrl, _ := url.Parse("syslog-tls://localhost")
			_, err := syslogwriter.NewHttpsWriter(outputUrl, "appId", false)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for syslog scheme", func() {
			outputUrl, _ := url.Parse("syslog://localhost")
			_, err := syslogwriter.NewHttpsWriter(outputUrl, "appId", false)
			Expect(err).To(HaveOccurred())
		})
	})
})

func ServeHTTP(requestChan chan []byte) *httptest.Server {
	handler := func(_ http.ResponseWriter, r *http.Request) {
		bytes := make([]byte, 1024)
		byteCount, _ := r.Body.Read(bytes)
		r.Body.Close()
		requestChan <- bytes[:byteCount]
	}

	http.HandleFunc("/234-bxg-234/", handler)

	server := httptest.NewTLSServer(http.DefaultServeMux)
	return server
}
