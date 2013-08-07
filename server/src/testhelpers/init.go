package testhelpers

import (
	"code.google.com/p/go.net/websocket"
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/stretchr/testify/assert"
	"logMessage"
	"testing"
	"time"
)

func AddWSSink(t *testing.T, receivedChan chan []byte, port string, path string, authToken string) (*websocket.Conn, chan bool) {
	dontKeepAliveChan := make(chan bool)

	config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
	assert.NoError(t, err)
	config.Header.Add("Authorization", authToken)
	ws, err := websocket.DialConfig(config)
	assert.NoError(t, err)

	go func() {
		for {
			var data []byte
			err := websocket.Message.Receive(ws, &data)
			if err != nil {
				break
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
			case <-time.After(44 * time.Millisecond):
				// keep-alive
			}
		}
	}()
	return ws, dontKeepAliveChan
}

func MarshalledLogMessage(t *testing.T, messageString string, spaceId string, appId string) []byte {
	currentTime := time.Now()

	messageType := logMessage.LogMessage_OUT
	sourceType := logMessage.LogMessage_DEA
	protoMessage := &logMessage.LogMessage{
		Message:     []byte(messageString),
		AppId:       proto.String(appId),
		SpaceId:     proto.String(spaceId),
		MessageType: &messageType,
		SourceType:  &sourceType,
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	message, err := proto.Marshal(protoMessage)
	assert.NoError(t, err)

	return message
}

func AssertProtoBufferMessageEquals(t *testing.T, expectedMessage string, actual []byte) {
	receivedMessage := &logMessage.LogMessage{}

	err := proto.Unmarshal(actual, receivedMessage)
	assert.NoError(t, err)
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
}
