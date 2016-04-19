package helpers

import (
	"crypto/sha1"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/signature"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"

	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func SendAppLog(appID string, message string, connection net.Conn) error {
	logMessage := factories.NewLogMessage(events.LogMessage_OUT, message, appID, "APP")

	return SendEvent(logMessage, connection)
}

func SendEvent(event events.Event, connection net.Conn) error {
	signedEnvelope := MarshalEvent(event, "secret")

	_, err := connection.Write(signedEnvelope)
	return err
}

func MarshalEvent(event events.Event, secret string) []byte {
	envelope, _ := emitter.Wrap(event, "origin")
	envelopeBytes := MarshalProtoBuf(envelope)

	return signature.SignMessage(envelopeBytes, []byte(secret))
}

func MarshalProtoBuf(pb proto.Message) []byte {
	marshalledProtoBuf, err := proto.Marshal(pb)
	Expect(err).NotTo(HaveOccurred())

	return marshalledProtoBuf
}

func AddWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, <-chan struct{}) {
	connectionDroppedChannel := make(chan struct{}, 1)

	var ws *websocket.Conn

	ip := "127.0.0.1"
	fullURL := "ws://" + ip + ":" + port + path

	Eventually(func() error {
		var err error
		ws, _, err = websocket.DefaultDialer.Dial(fullURL, http.Header{})
		return err
	}, 5, 1).ShouldNot(HaveOccurred(), fmt.Sprintf("Unable to connect to server at %s.", fullURL))

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

func DecodeProtoBufEnvelope(actual []byte) *events.Envelope {
	var receivedEnvelope events.Envelope
	err := proto.Unmarshal(actual, &receivedEnvelope)
	Expect(err).NotTo(HaveOccurred())
	return &receivedEnvelope
}

func DecodeProtoBufLogMessage(actual []byte) *events.LogMessage {
	receivedEnvelope := DecodeProtoBufEnvelope(actual)
	return receivedEnvelope.GetLogMessage()
}

func DecodeProtoBufCounterEvent(actual []byte) *events.CounterEvent {
	receivedEnvelope := DecodeProtoBufEnvelope(actual)
	return receivedEnvelope.GetCounterEvent()
}

func AppKey(appID string) string {
	return fmt.Sprintf("/loggregator/services/%s", appID)
}

func DrainKey(appID string, drainURL string) string {
	hash := sha1.Sum([]byte(drainURL))
	return fmt.Sprintf("%s/%x", AppKey(appID), hash)
}

func AddETCDNode(etcdAdapter storeadapter.StoreAdapter, key string, value string) {
	node := storeadapter.StoreNode{
		Key:   key,
		Value: []byte(value),
		TTL:   uint64(20),
	}
	etcdAdapter.Create(node)
	recvNode, err := etcdAdapter.Get(key)
	Expect(err).NotTo(HaveOccurred())
	Expect(string(recvNode.Value)).To(Equal(value))
}

func UnmarshalMessage(messageBytes []byte) events.Envelope {
	var envelope events.Envelope
	err := proto.Unmarshal(messageBytes, &envelope)
	Expect(err).NotTo(HaveOccurred())
	return envelope
}

func StartHTTPSServer(pathToHTTPEchoServer string) *gexec.Session {
	command := exec.Command(pathToHTTPEchoServer, "-cert", "../fixtures/server.crt", "-key", "../fixtures/server.key")
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return session
}

func StartUnencryptedTCPServer(pathToTCPEchoServer string, syslogDrainAddress string) *gexec.Session {
	command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress)
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return drainSession
}

func StartEncryptedTCPServer(pathToTCPEchoServer string, syslogDrainAddress string) *gexec.Session {
	command := exec.Command(pathToTCPEchoServer, "-address", syslogDrainAddress, "-ssl", "-cert", "../fixtures/server.crt", "-key", "../fixtures/server.key")
	drainSession, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return drainSession
}
