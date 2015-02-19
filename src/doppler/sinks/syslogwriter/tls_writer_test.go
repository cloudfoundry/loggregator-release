package syslogwriter_test

import (
	"crypto/tls"
	"doppler/sinks/syslogwriter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/url"
	"sync"
	"time"
)

var _ = Describe("TlsWriter", func() {

	var shutdownChan chan bool
	var serverStoppedChan <-chan bool
	standardErrorPriority := 14

	BeforeEach(func() {
		shutdownChan = make(chan bool)
		serverStoppedChan = startTLSSyslogServer(shutdownChan)
	})

	AfterEach(func() {
		close(shutdownChan)
		<-serverStoppedChan
	})

	It("connects", func() {
		outputUrl, _ := url.Parse("syslog-tls://localhost:9999")
		w, _ := syslogwriter.NewTlsWriter(outputUrl, "appId", true)
		err := w.Connect()
		Expect(err).To(BeNil())
		_, err = w.Write(standardErrorPriority, []byte("just a test"), "test", "", time.Now().UnixNano())
		Expect(err).To(BeNil())
		w.Close()
	})

	It("rejects self-signed certs", func() {
		outputUrl, _ := url.Parse("syslog-tls://localhost:9999")
		w, _ := syslogwriter.NewTlsWriter(outputUrl, "appId", false)
		err := w.Connect()
		Expect(err).ToNot(BeNil())
	})

	It("returns an error for syslog scheme", func() {
		outputUrl, _ := url.Parse("syslog://localhost:9999")
		_, err := syslogwriter.NewTlsWriter(outputUrl, "appId", false)
		Expect(err).To(HaveOccurred())
	})

	It("returns an error for https scheme", func() {
		outputUrl, _ := url.Parse("https://localhost:9999")
		_, err := syslogwriter.NewTlsWriter(outputUrl, "appId", false)
		Expect(err).To(HaveOccurred())
	})
})

func startTLSSyslogServer(shutdownChan <-chan bool) <-chan bool {
	doneChan := make(chan bool)
	cert, err := tls.X509KeyPair(localhostCert, localhostKey)
	if err != nil {
		panic(err)
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", "localhost:9999", config)

	var listenerStopped sync.WaitGroup
	listenerStopped.Add(1)

	if err != nil {
		panic(err)
	}

	go func() {
		<-shutdownChan
		listener.Close()
		listenerStopped.Wait()
		close(doneChan)
	}()

	go func() {
		defer listenerStopped.Done()
		buffer := make([]byte, 1024)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		conn.Read(buffer)
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
