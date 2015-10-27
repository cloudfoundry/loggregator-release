package websocketserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"net/http"
	"testing"
	"time"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
)

var logger = loggertesthelper.Logger()

func TestWebsocketServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WebsocketServer Suite")
}

func AddWSSink(receivedChan chan []byte, url string) (chan struct{}, <-chan struct{}, error) {
	stopKeepAlive := make(chan struct{})
	connectionDropped := make(chan struct{})

	ws, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
	if err != nil {
		return nil, nil, err
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

	return stopKeepAlive, connectionDropped, nil
}

func parseEnvelope(actual []byte) (*events.Envelope, error) {
	receivedMessage := &events.Envelope{}
	err := proto.Unmarshal(actual, receivedMessage)
	return receivedMessage, err
}
