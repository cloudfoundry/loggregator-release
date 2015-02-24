package doppler_test

import (
	"crypto/sha1"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/storeadapter"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/pivotal-golang/localip"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func sendAppLog(appID string, message string, connection net.Conn) error {
	logMessage := factories.NewLogMessage(events.LogMessage_OUT, message, appID, "APP")

	return sendEvent(logMessage, connection)
}

func sendEvent(event events.Event, connection net.Conn) error {
	signedEnvelope := marshalEvent(event, "secret")

	_, err := connection.Write(signedEnvelope)
	return err
}

func marshalEvent(event events.Event, secret string) []byte {
	envelope, _ := emitter.Wrap(event, "origin")
	envelopeBytes := marshalProtoBuf(envelope)

	return signature.SignMessage(envelopeBytes, []byte(secret))
}

func marshalProtoBuf(pb proto.Message) []byte {
	marshalledProtoBuf, err := proto.Marshal(pb)
	if err != nil {
		Fail(err.Error())
	}

	return marshalledProtoBuf
}

func addWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, <-chan struct{}) {
	connectionDroppedChannel := make(chan struct{}, 1)

	var ws *websocket.Conn

	ip, _ := localip.LocalIP()
	fullURL := "ws://" + ip + ":" + port + path

	Eventually(func() error {
		var err error
		ws, _, err = websocket.DefaultDialer.Dial(fullURL, http.Header{})
		return err
	}).ShouldNot(HaveOccurred(), fmt.Sprintf("Unable to connect to server at %s.", fullURL))

	ws.SetPingHandler(func(message string) error {
		ws.WriteControl(websocket.PongMessage, []byte(message), time.Time{})
		return nil
	})

	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				close(connectionDroppedChannel)
				close(receivedChan)
				return
			}

			receivedChan <- data
		}

	}()

	return ws, connectionDroppedChannel
}

func decodeProtoBufLogMessage(actual []byte) *events.LogMessage {
	var receivedEnvelope events.Envelope
	err := proto.Unmarshal(actual, &receivedEnvelope)
	if err != nil {
		Fail(err.Error())
	}
	return receivedEnvelope.GetLogMessage()
}

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
