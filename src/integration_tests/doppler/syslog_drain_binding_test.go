package doppler_test

import (
	"crypto/sha1"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/cloudfoundry/storeadapter"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Syslog Drain Binding", func() {

	var (
		appID           string
		inputConnection net.Conn
		shutdownChan    chan struct{}
	)

	BeforeEach(func() {
		guid, _ := uuid.NewV4()
		appID = guid.String()
		inputConnection, _ = net.Dial("udp", localIPAddress+":8765")
	})

	AfterEach(func() {
		inputConnection.Close()
	})

	Context("when connecting over TCP", func() {
		var (
			syslogDrainAddress string
			dataChan           <-chan []byte
			serverStoppedChan  <-chan struct{}
		)

		BeforeEach(func() {
			syslogPort := 6666
			syslogDrainAddress = fmt.Sprintf("%s:%d", localIPAddress, syslogPort)

			shutdownChan = make(chan struct{})
		})

		AfterEach(func() {
			close(shutdownChan)
			<-serverStoppedChan
		})

		Context("when forwarding to an unencrypted syslog:// endpoint", func() {
			BeforeEach(func() {
				dataChan, serverStoppedChan = startSyslogServer(shutdownChan, syslogDrainAddress)
			})

			It("forwards log messages to a syslog", func() {
				syslogDrainURL := "syslog://" + syslogDrainAddress
				key := drainKey(appID, syslogDrainURL)
				addETCDNode(key, syslogDrainURL)

				Eventually(func() string {
					sendAppLog(appID, "standard syslog msg", inputConnection)
					select {
					case message := <-dataChan:
						return string(message)
					default:
						return ""
					}

				}, 90, 1).Should(ContainSubstring("standard syslog msg"))
			})
		})

		Context("when forwarding to an encrypted syslog-tls:// endpoint", func() {
			BeforeEach(func() {
				dataChan, serverStoppedChan = startSyslogTLSServer(shutdownChan, syslogDrainAddress)
			})

			It("forwards log messages to a syslog-tls", func() {
				syslogDrainURL := "syslog-tls://" + syslogDrainAddress
				key := drainKey(appID, syslogDrainURL)
				addETCDNode(key, syslogDrainURL)

				Eventually(func() string {
					sendAppLog(appID, "tls-message", inputConnection)
					select {
					case message := <-dataChan:
						return string(message)
					default:
						return ""
					}
				}, 90, 1).Should(ContainSubstring("tls-message"))
			})
		})

	})

	Context("when forwarding to an https:// endpoint", func() {
		var (
			server      *httptest.Server
			requestChan chan []byte
		)

		BeforeEach(func() {
			requestChan = make(chan []byte, 1)
			server, shutdownChan = startHTTPSServer(requestChan)
		})

		AfterEach(func() {
			close(shutdownChan)
			server.Close()
			close(requestChan)
		})

		It("forwards log messages to an https endpoint", func() {
			httpsURL := server.URL + "/234-bxg-234/"
			key := drainKey(appID, httpsURL)
			addETCDNode(key, httpsURL)

			Eventually(func() string {
				sendAppLog(appID, "http-message", inputConnection)
				select {
				case message := <-requestChan:
					return string(message)
				default:
					return ""
				}
			}, 10, 1).Should(ContainSubstring("http-message"))
		})
	})
})

func startSyslogServer(shutdownChan <-chan struct{}, address string) (<-chan []byte, <-chan struct{}) {
	doneChan := make(chan struct{})
	dataChan := make(chan []byte)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}

	var listenerStopped sync.WaitGroup
	listenerStopped.Add(1)

	go func() {
		<-shutdownChan
		listener.Close()
		listenerStopped.Wait()
		close(doneChan)
	}()

	go func() {
		var connectionsDone sync.WaitGroup

		defer listenerStopped.Done()
		defer connectionsDone.Wait()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			connectionsDone.Add(1)
			go func() {
				defer conn.Close()
				defer connectionsDone.Done()
				buffer := make([]byte, 1024)
				for {
					select {
					case <-shutdownChan:
						return
					default:
					}

					conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
					readCount, err := conn.Read(buffer)
					if err == io.EOF {
						return
					} else if err != nil {
						continue
					}

					buffer2 := make([]byte, readCount)
					copy(buffer2, buffer[:readCount])
					dataChan <- buffer2
				}
			}()
		}
	}()

	var testConn net.Conn
	Eventually(func() error {
		testConn, err = net.Dial("tcp", address)
		return err
	}).ShouldNot(HaveOccurred())

	testConn.Close()

	return dataChan, doneChan
}

func startSyslogTLSServer(shutdownChan <-chan struct{}, address string) (<-chan []byte, <-chan struct{}) {
	doneChan := make(chan struct{})
	dataChan := make(chan []byte)

	cert, err := tls.X509KeyPair(localhostCert, localhostKey)
	if err != nil {
		panic(err)
	}
	config := &tls.Config{
		InsecureSkipVerify:     true,
		Certificates:           []tls.Certificate{cert},
		SessionTicketsDisabled: true,
	}

	listener, err := tls.Listen("tcp", address, config)
	if err != nil {
		panic(err)
	}

	var listenerStopped sync.WaitGroup
	listenerStopped.Add(1)

	go func() {
		<-shutdownChan
		listener.Close()
		listenerStopped.Wait()
		close(doneChan)
	}()

	go func() {
		var connectionsDone sync.WaitGroup

		defer listenerStopped.Done()
		defer connectionsDone.Wait()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			connectionsDone.Add(1)
			go func() {
				defer conn.Close()
				defer connectionsDone.Done()
				buffer := make([]byte, 1024)
				for {
					select {
					case <-shutdownChan:
						return
					default:
					}

					conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
					readCount, err := conn.Read(buffer)
					if err == io.EOF {
						return
					} else if err != nil {
						continue
					}

					buffer2 := make([]byte, readCount)
					copy(buffer2, buffer[:readCount])
					dataChan <- buffer2
				}
			}()
		}
	}()

	var testConn net.Conn
	Eventually(func() error {
		testConn, err = tls.Dial("tcp", address, config)
		return err
	}).ShouldNot(HaveOccurred())

	testConn.Close()

	return dataChan, doneChan
}

func startHTTPSServer(requestChan chan []byte) (*httptest.Server, chan struct{}) {
	doneChan := make(chan struct{})
	handler := func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-doneChan:
			return
		default:
		}

		bytes := make([]byte, 1024)
		byteCount, _ := r.Body.Read(bytes)
		r.Body.Close()
		requestChan <- bytes[:byteCount]
	}

	http.HandleFunc("/234-bxg-234/", handler)

	server := httptest.NewTLSServer(http.DefaultServeMux)
	return server, doneChan
}

func appKey(appID string) string {
	return fmt.Sprintf("/loggregator/services/%s", appID)
}

func drainKey(appID string, drainURL string) string {
	hash := sha1.Sum([]byte(drainURL))
	return fmt.Sprintf("%s/%x", appKey(appID), hash)
}

func addETCDNode(key string, value string) {
	adapter := etcdRunner.Adapter()

	node := storeadapter.StoreNode{
		Key:   key,
		Value: []byte(value),
		TTL:   uint64(2),
	}
	adapter.Create(node)
	recvNode, err := adapter.Get(key)
	Expect(err).NotTo(HaveOccurred())
	Expect(string(recvNode.Value)).To(Equal(value))
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
