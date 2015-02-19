package doppler_test

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cloudfoundry/dropsonde/emitter"
	"github.com/cloudfoundry/dropsonde/events"
	"github.com/cloudfoundry/dropsonde/factories"
	"github.com/cloudfoundry/dropsonde/signature"
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
