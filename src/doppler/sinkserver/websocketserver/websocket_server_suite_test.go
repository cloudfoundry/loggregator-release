package websocketserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
)

func TestWebsocketServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WebsocketServer Suite")
}

func addSlowWSSink(receivedChan chan []byte, errChan chan error, timeout time.Duration, url string) {
	ws, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		time.Sleep(timeout)
		_, reader, err := ws.NextReader()
		if err != nil {
			errChan <- err
			return
		}
		received, err := ioutil.ReadAll(reader)
		if err != nil {
			errChan <- err
			return
		}
		receivedChan <- received
	}()
}

func addWSSink(receivedChan chan []byte, url string) (chan struct{}, <-chan struct{}, func(), error) {
	stopKeepAlive := make(chan struct{})
	connectionDropped := make(chan struct{})

	ws, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
	if err != nil {
		return nil, nil, func() {}, err
	}

	ws.SetPingHandler(func(message string) error {
		select {
		case <-stopKeepAlive:
			return nil
		default:
			return ws.WriteControl(websocket.PongMessage, []byte(message), time.Time{})
		}
	})

	go func() {
		defer close(connectionDropped)
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			receivedChan <- data
		}
	}()

	return stopKeepAlive, connectionDropped, func() {
		ws.Close()
	}, nil
}

func parseEnvelope(actual []byte) (*events.Envelope, error) {
	receivedMessage := &events.Envelope{}
	err := proto.Unmarshal(actual, receivedMessage)
	return receivedMessage, err
}
