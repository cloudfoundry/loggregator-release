package syslogwriter_test

import (
	"doppler/sinks/syslogwriter"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const standardErrorPriority = 14

var _ = Describe("HttpsWriter", func() {

	Context("With an HTTPS Sink", func() {
		var server *httptest.Server
		var requestChan chan []byte
		var dialer *net.Dialer
		var timeout time.Duration
		var queuedRequests int

		BeforeEach(func() {
			timeout = 0
			queuedRequests = 1
		})

		JustBeforeEach(func() {
			requestChan = make(chan []byte, queuedRequests)
			server = ServeHTTP(requestChan)
			dialer = &net.Dialer{Timeout: 1 * time.Second}
		})

		AfterEach(func() {
			server.Close()
			http.DefaultServeMux = http.NewServeMux()
		})

		It("HTTP POSTs each log message to the HTTPS syslog endpoint", func() {
			outputUrl, _ := url.Parse(server.URL + "/234-bxg-234/")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
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

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
			_, err := w.Write(standardErrorPriority, []byte("Message"), "just a test", "TEST", time.Now().UnixNano())
			Expect(err).To(HaveOccurred())
		})

		It("holds onto the last error when unable to POST a log message", func() {
			outputUrl, _ := url.Parse("https://")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
			_, err := w.Write(standardErrorPriority, []byte("Message"), "just a test", "TEST", time.Now().UnixNano())

			conErr := w.Connect()
			Expect(conErr).To(Equal(err))
			Expect(w.Connect()).To(BeNil())
		})

		It("should close connections and return an error if status code returned is not 200", func() {
			outputUrl, _ := url.Parse(server.URL + "/doesnotexist")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
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

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
			err := w.Connect()
			Expect(err).ToNot(HaveOccurred())

			parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
			_, err = w.Write(standardErrorPriority, []byte("Message"), "just a test", "TEST", parsedTime.UnixNano())
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the target sink is slow to accept connections", func() {
			var listener net.Listener

			BeforeEach(func() {
				var err error
				listener, err = net.Listen("tcp", "127.0.0.1:0")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				listener.Close()
			})

			It("times out", func() {
				outputUrl, _ := url.Parse("https://" + listener.Addr().String() + "/")
				w, err := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
				Expect(err).NotTo(HaveOccurred())

				err = w.Connect()
				Expect(err).NotTo(HaveOccurred())

				parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
				_, err = w.Write(standardErrorPriority, []byte("Message"), "just a test", "TEST", parsedTime.UnixNano())
				Expect(err).To(BeAssignableToTypeOf(&url.Error{}))
				urlErr := err.(*url.Error)
				Expect(urlErr.Err).To(MatchError("net/http: TLS handshake timeout"))
			})
		})

		Context("when a request times out", func() {
			BeforeEach(func() {
				queuedRequests = 1
				timeout = 500 * time.Millisecond
			})

			JustBeforeEach(func() {
				requestChan <- []byte{}
			})

			AfterEach(func() {
				<-requestChan
				<-requestChan
			})

			It("returns a timeout error", func() {
				outputUrl, _ := url.Parse(server.URL + "/234-bxg-234/")
				w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
				err := w.Connect()
				Expect(err).ToNot(HaveOccurred())

				parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
				_, err = w.Write(standardErrorPriority, []byte("Message"), "just a test", "TEST", parsedTime.UnixNano())
				Expect(err).To(HaveOccurred())
			})
		})

		It("returns an error for syslog-tls scheme", func() {
			outputUrl, _ := url.Parse("syslog-tls://localhost")
			_, err := syslogwriter.NewHttpsWriter(outputUrl, "appId", false, dialer, timeout)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for syslog scheme", func() {
			outputUrl, _ := url.Parse("syslog://localhost")
			_, err := syslogwriter.NewHttpsWriter(outputUrl, "appId", false, dialer, timeout)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when the provided dialer is nil", func() {
			outputURL, _ := url.Parse("https://localhost")
			_, err := syslogwriter.NewHttpsWriter(outputURL, "appId", false, nil, timeout)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot construct a writer with a nil dialer"))
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
