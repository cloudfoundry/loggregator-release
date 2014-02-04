package server_testhelpers

import (
	"code.google.com/p/go.net/websocket"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"time"
)

func AddWSSink(t assert.TestingT, receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool, <-chan bool) {
	dontKeepAliveChan := make(chan bool, 1)
	connectionDroppedChannel := make(chan bool, 1)

	config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
	assert.NoError(t, err)
	ws, err := websocket.DialConfig(config)
	assert.NoError(t, err)

	go func() {
		for {
			var data []byte
			err := websocket.Message.Receive(ws, &data)
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
			err := websocket.Message.Send(ws, []byte{42})
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
