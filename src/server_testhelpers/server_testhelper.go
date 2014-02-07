package server_testhelpers

import (
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"net/http"
	"time"
)

func AddWSSink(t assert.TestingT, receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool, <-chan bool) {
	dontKeepAliveChan := make(chan bool, 1)
	connectionDroppedChannel := make(chan bool, 1)

	ws, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})
	assert.NoError(t, err)

	go func() {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				connectionDroppedChannel <- true
				close(receivedChan)
				return
			}
			receivedChan <- data
		}

	}()
	go func() {
		for {
			err := ws.WriteMessage(websocket.BinaryMessage, []byte{42})
			if err != nil {
				break
			}
			select {
			case <-dontKeepAliveChan:
				return
			case <-time.After(4 * time.Millisecond):
				// keep-alive
			}
		}
	}()
	return ws, dontKeepAliveChan, connectionDroppedChannel
}

func UnmarshallerMaker(secret string) func([]byte) (*logmessage.Message, error) {
	return func(data []byte) (*logmessage.Message, error) {
		return logmessage.ParseEnvelope(data, secret)
	}
}
