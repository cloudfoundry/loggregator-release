package syslogwriter_test

import (
	"crypto/tls"
	"doppler/sinks/syslogwriter"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"time"
)

var _ = Describe("SyslogWriter", func() {
	Context("With syslog Connection", func() {

		var dataChan <-chan []byte
		var serverStoppedChan <-chan bool
		var shutdownChan chan bool
		var sysLogWriter syslogwriter.SyslogWriter

		BeforeEach(func() {
			shutdownChan = make(chan bool)
			dataChan, serverStoppedChan = startSyslogServer(shutdownChan)
			outputUrl, _ := url.Parse("syslog://localhost:9999")
			sysLogWriter = syslogwriter.NewSyslogWriter(outputUrl, "appId", false)
			sysLogWriter.Connect()
		})

		AfterEach(func() {
			close(shutdownChan)
			sysLogWriter.Close()
			<-serverStoppedChan
		})

		Context("Message Format", func() {
			It("sends messages from stdout with INFO priority", func(done Done) {
				sysLogWriter.WriteStdout([]byte("just a test"), "test", "", time.Now().UnixNano())

				data := <-dataChan
				Expect(string(data)).To(MatchRegexp(`\d <14>\d `))
				close(done)
			})

			It("sends messages from stderr with ERROR priority", func(done Done) {
				sysLogWriter.WriteStderr([]byte("just a test"), "test", "", time.Now().UnixNano())

				data := <-dataChan
				Expect(string(data)).To(MatchRegexp(`\d <11>\d `))
				close(done)
			})

			It("sends messages in the proper format", func(done Done) {
				sysLogWriter.WriteStdout([]byte("just a test"), "App", "2", time.Now().UnixNano())

				data := <-dataChan
				Expect(string(data)).To(MatchRegexp(`\d <\d+>1 \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{1,6}([-+]\d{2}:\d{2}) loggregator appId \[App/2\] - - just a test\n`))
				close(done)
			})

			It("strips null termination char from message", func(done Done) {
				sysLogWriter.WriteStdout([]byte(string(0)+" hi"), "appId", "", time.Now().UnixNano())

				data := <-dataChan
				Expect(string(data)).ToNot(MatchRegexp("\000"))
				close(done)
			})
		})
	})

	Context("With an HTTPS Connection", func() {

		var server *httptest.Server
		var requestChan chan []byte

		BeforeEach(func() {
			requestChan = make(chan []byte, 1)
			server = ServeHTTP(requestChan, loggertesthelper.Logger())
		})

		AfterEach(func() {
			server.Close()
			http.DefaultServeMux = http.NewServeMux()
		})

		It("HTTP POSTs each log message to the HTTPS syslog endpoint", func() {
			outputUrl, _ := url.Parse(server.URL + "/234-bxg-234/")

			w := syslogwriter.NewSyslogWriter(outputUrl, "appId", true)
			err := w.Connect()
			Expect(err).ToNot(HaveOccurred())

			parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
			byteCount, err := w.WriteStdout([]byte("Message"), "just a test", "TEST", parsedTime.UnixNano())
			Expect(byteCount).To(Equal(79))
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() string {
				return string(<-requestChan)
			}).Should(ContainSubstring("loggregator appId [just a test] - - Message"))
		})

		It("returns an error when unable to HTTP POST the log message", func() {
			outputUrl, _ := url.Parse("https://")

			w := syslogwriter.NewSyslogWriter(outputUrl, "appId", true)

			_, err := w.WriteStdout([]byte("Message"), "just a test", "TEST", time.Now().UnixNano())
			Expect(err).To(HaveOccurred())
		})

		It("should close connections and return an error if status code returned is not 200", func() {
			outputUrl, _ := url.Parse(server.URL + "/doesnotexist")

			w := syslogwriter.NewSyslogWriter(outputUrl, "appId", true)
			err := w.Connect()
			Expect(err).ToNot(HaveOccurred())

			parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
			for i := 0; i < 10; i++ {
				_, err := w.WriteStdout([]byte("Message"), "just a test", "TEST", parsedTime.UnixNano())
				Expect(err).To(HaveOccurred())
			}

			origNumGoRoutines := runtime.NumGoroutine()
			Eventually(runtime.NumGoroutine, 6).Should(BeNumerically("<", origNumGoRoutines))
		})

		It("should not return error for response 200 status codes", func() {
			outputUrl, _ := url.Parse(server.URL + "/234-bxg-234/")

			w := syslogwriter.NewSyslogWriter(outputUrl, "appId", true)
			err := w.Connect()
			Expect(err).ToNot(HaveOccurred())

			parsedTime, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
			_, err = w.WriteStdout([]byte("Message"), "just a test", "TEST", parsedTime.UnixNano())
			Expect(err).ToNot(HaveOccurred())

		})
	})

	Context("With TLS Connection", func() {

		var shutdownChan chan bool
		var serverStoppedChan <-chan bool

		BeforeEach(func() {
			shutdownChan = make(chan bool)
			serverStoppedChan = startTLSSyslogServer(shutdownChan, loggertesthelper.Logger())
		})

		AfterEach(func() {
			close(shutdownChan)
			<-serverStoppedChan
		})

		It("connects", func() {
			outputUrl, _ := url.Parse("syslog-tls://localhost:9999")
			w := syslogwriter.NewSyslogWriter(outputUrl, "appId", true)
			err := w.Connect()
			Expect(err).To(BeNil())
			_, err = w.WriteStdout([]byte("just a test"), "test", "", time.Now().UnixNano())
			Expect(err).To(BeNil())
			w.Close()
		})

		It("rejects self-signed certs", func() {
			outputUrl, _ := url.Parse("syslog-tls://localhost:9999")
			w := syslogwriter.NewSyslogWriter(outputUrl, "appId", false)
			err := w.Connect()
			Expect(err).ToNot(BeNil())
		})
	})
})

func ServeHTTP(requestChan chan []byte, logger *gosteno.Logger) *httptest.Server {
	handler := func(w http.ResponseWriter, r *http.Request) {
		bytes := make([]byte, 1024)
		byteCount, _ := r.Body.Read(bytes)
		r.Body.Close()
		requestChan <- bytes[:byteCount]
	}

	http.HandleFunc("/234-bxg-234/", handler)

	server := httptest.NewTLSServer(http.DefaultServeMux)
	return server
}

func startSyslogServer(shutdownChan <-chan bool) (<-chan []byte, <-chan bool) {
	dataChan := make(chan []byte)
	doneChan := make(chan bool)
	listener, err := net.Listen("tcp", "localhost:9999")
	if err != nil {
		panic(err)
	}

	go func() {
		<-shutdownChan
		listener.Close()
		close(doneChan)
	}()

	go func() {
		buffer := make([]byte, 1024)
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		readCount, err := conn.Read(buffer)
		buffer2 := make([]byte, readCount)
		copy(buffer2, buffer[:readCount])
		dataChan <- buffer2
		conn.Close()
	}()

	<-time.After(300 * time.Millisecond)
	return dataChan, doneChan
}

func startTLSSyslogServer(shutdownChan <-chan bool, logger *gosteno.Logger) <-chan bool {
	doneChan := make(chan bool)
	cert, err := tls.X509KeyPair(localhostCert, localhostKey)
	if err != nil {
		panic(err)
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", "localhost:9999", config)

	if err != nil {
		panic(err)
	}

	go func() {
		<-shutdownChan
		listener.Close()
		close(doneChan)
	}()

	go func() {
		buffer := make([]byte, 1024)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		_, err = conn.Read(buffer)

		conn.Close()
	}()

	<-time.After(300 * time.Millisecond)
	return doneChan
}

var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBdzCCASOgAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw03MDAxMDEwMDAwMDBaFw00OTEyMzEyMzU5NTlaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wWjALBgkqhkiG9w0BAQEDSwAwSAJBAN55NcYKZeInyTuhcCwFMhDHCmwa
IUSdtXdcbItRB/yfXGBhiex00IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEA
AaNoMGYwDgYDVR0PAQH/BAQDAgCkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1Ud
EwEB/wQFMAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAA
AAAAAAAAAAAAAAEwCwYJKoZIhvcNAQEFA0EAAoQn/ytgqpiLcZu9XKbCJsJcvkgk
Se6AbGXgSlq+ZCEVo0qIwSgeBqmsJxUu7NCSOwVJLYNEBO2DtIxoYVk+MA==
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPAIBAAJBAN55NcYKZeInyTuhcCwFMhDHCmwaIUSdtXdcbItRB/yfXGBhiex0
0IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEAAQJBAQdUx66rfh8sYsgfdcvV
NoafYpnEcB5s4m/vSVe6SU7dCK6eYec9f9wpT353ljhDUHq3EbmE4foNzJngh35d
AekCIQDhRQG5Li0Wj8TM4obOnnXUXf1jRv0UkzE9AHWLG5q3AwIhAPzSjpYUDjVW
MCUXgckTpKCuGwbJk7424Nb8bLzf3kllAiA5mUBgjfr/WtFSJdWcPQ4Zt9KTMNKD
EUO0ukpTwEIl6wIhAMbGqZK3zAAFdq8DD2jPx+UJXnh0rnOkZBzDtJ6/iN69AiEA
1Aq8MJgTaYsDQWyU/hDq5YkDJc9e9DSCvUIzqxQWMQE=
-----END RSA PRIVATE KEY-----`)
