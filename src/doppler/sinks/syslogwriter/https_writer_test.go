package syslogwriter_test

import (
	"doppler/sinks/syslogwriter"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const standardErrorPriority = 14

var _ = Describe("HttpsWriter", func() {

	Context("With an HTTPS Sink", func() {
		var server *httptest.Server
		var listener *historyListener
		var serveMux *http.ServeMux
		var requestChan chan []byte
		var dialer *net.Dialer
		var timeout time.Duration
		var queuedRequests int
		var statusCode int

		BeforeEach(func() {
			listener = newHistoryListener("tcp", "127.0.0.1:0")
			serveMux = http.NewServeMux()
			server = httptest.NewUnstartedServer(serveMux)
			dialer = &net.Dialer{Timeout: 1 * time.Second}
			timeout = 0
			queuedRequests = 1
			statusCode = http.StatusOK
		})

		JustBeforeEach(func() {
			requestChan = make(chan []byte, queuedRequests)
			serveMux.HandleFunc("/234-bxg-234/", syslogHandler(requestChan, statusCode))
			server.Listener = listener
			server.StartTLS()
		})

		AfterEach(func() {
			server.Close()
		})

		It("HTTP POSTs each log message to the HTTPS syslog endpoint", func() {
			outputUrl, _ := url.Parse(server.URL + "/234-bxg-234/")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
			err := w.Connect()
			Expect(err).ToNot(HaveOccurred())

			parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
			_, err = w.Write(standardErrorPriority, []byte("Message"), "test", "TEST", parsedTime.UnixNano())
			Expect(err).ToNot(HaveOccurred())
			Eventually(requestChan).Should(Receive(ContainSubstring("loggregator appId [TEST] - - Message")))
		})

		It("returns an error when unable to HTTP POST the log message", func() {
			outputUrl, _ := url.Parse("https://")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
			_, err := w.Write(standardErrorPriority, []byte("Message"), "test", "TEST", time.Now().UnixNano())
			Expect(err).To(HaveOccurred())
		})

		It("holds onto the last error when unable to POST a log message", func() {
			outputUrl, _ := url.Parse("https://")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
			_, err := w.Write(standardErrorPriority, []byte("Message"), "test", "TEST", time.Now().UnixNano())

			conErr := w.Connect()
			Expect(conErr).To(Equal(err))
			Expect(w.Connect()).To(BeNil())
		})

		It("should close connections and return an error if status code returned is not 2XX", func() {
			outputUrl, _ := url.Parse(server.URL + "/doesnotexist")

			w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
			err := w.Connect()
			Expect(err).ToNot(HaveOccurred())

			parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
			for i := 0; i < 10; i++ {
				_, err := w.Write(standardErrorPriority, []byte("Message"), "test", "TEST", parsedTime.UnixNano())
				Expect(err).To(HaveOccurred())
			}
		})

		Context("returned status code is 2XX (but not 200)", func() {
			BeforeEach(func() {
				statusCode = 294
			})

			It("should not return error for response 2XX status codes", func() {
				outputUrl, _ := url.Parse(server.URL + "/234-bxg-234/")

				w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
				err := w.Connect()
				Expect(err).ToNot(HaveOccurred())

				parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
				_, err = w.Write(standardErrorPriority, []byte("Message"), "test", "TEST", parsedTime.UnixNano())
				Expect(err).ToNot(HaveOccurred())
			})
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
				_, err = w.Write(standardErrorPriority, []byte("Message"), "test", "TEST", parsedTime.UnixNano())
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
				_, err = w.Write(standardErrorPriority, []byte("Message"), "test", "TEST", parsedTime.UnixNano())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when requests complete", func() {
			var requester *concurrentWriteRequestSimulator

			BeforeEach(func() {
				requester = &concurrentWriteRequestSimulator{}
				serveMux.Handle("/pause/", requester)
			})

			It("only keeps one idle connection", func() {
				outputUrl, _ := url.Parse(server.URL + "/pause/")
				w, _ := syslogwriter.NewHttpsWriter(outputUrl, "appId", true, dialer, timeout)
				err := w.Connect()
				Expect(err).ToNot(HaveOccurred())

				// one connection per request
				requester.concurrentWriteRequests(2, w)
				Expect(listener.GetHistoryLength()).To(Equal(2))

				// one pooled connection, one new connection
				requester.concurrentWriteRequests(2, w)
				Expect(listener.GetHistoryLength()).To(BeNumerically("<=", 3))
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

func syslogHandler(requestChan chan []byte, statusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bytes := make([]byte, 1024)
		byteCount, _ := r.Body.Read(bytes)
		r.Body.Close()
		requestChan <- bytes[:byteCount]
		w.WriteHeader(statusCode)
	}
}

type historyListener struct {
	net.Listener
	sync.Mutex
	history []net.Conn
}

func newHistoryListener(network, address string) *historyListener {
	l, err := net.Listen(network, address)
	Expect(err).NotTo(HaveOccurred())

	return &historyListener{Listener: l}
}

func (h *historyListener) GetHistoryLength() int {
	defer h.Unlock()
	h.Lock()
	return len(h.history)
}

func (h *historyListener) Accept() (net.Conn, error) {
	c, err := h.Listener.Accept()
	if err == nil {
		h.Mutex.Lock()
		h.history = append(h.history, c)
		h.Mutex.Unlock()
	}
	return c, err
}

type concurrentWriteRequestSimulator struct {
}

func (c *concurrentWriteRequestSimulator) ServeHTTP(_ http.ResponseWriter, r *http.Request) {

}

func (c *concurrentWriteRequestSimulator) concurrentWriteRequests(count int, writer syslogwriter.Writer) {
	wg := &sync.WaitGroup{}
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			writer.Write(standardErrorPriority, []byte("Message"), "test", "TEST", time.Now().UnixNano())
			wg.Done()
		}()
	}

	wg.Wait()
}
