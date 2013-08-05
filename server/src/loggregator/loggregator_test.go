package loggregator

import (
	"code.google.com/p/go.net/websocket"
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"logMessage"
	"loggregator/agentlistener"
	"loggregator/sink"
	"net"
	"strings"
	"testing"
	"time"
)

func SuccessfulAuthorizer(a, b, c, d string, l *gosteno.Logger) bool {
	if b != "" {
		authString := strings.Split(b, " ")
		if len(authString) > 1 {
			return authString[1] == "correctAuthorizationToken"
		}
	}
	return false
}

func AssertProtoBufferMessageEquals(t *testing.T, expectedMessage string, actual []byte) {
	receivedMessage := &logMessage.LogMessage{}
	err := proto.Unmarshal(actual, receivedMessage)
	assert.NoError(t, err)
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
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

func AddWSSink(t *testing.T, receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool) {
	dontKeepAliveChan := make(chan bool)
	ws, err := websocket.Dial("ws://localhost:"+port+path, "", "http://localhost")
	assert.NoError(t, err)
	go func() {
		for {
			var data []byte
			err := websocket.Message.Receive(ws, &data)
			if err != nil {
				t.Logf("Error while ws sink is receiving data: %v\n", err)
				break
			}

			receivedChan <- data
		}

	}()
	go func() {
		for {
			err := websocket.Message.Send(ws, []byte{42})
			if err != nil {
				t.Logf("Error while ws sink is receiving data: %v\n", err)
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

func TestEndtoEndMessage(t *testing.T) {
	logger := gosteno.NewLogger("TestLogger")
	listener := agentlistener.NewAgentListener("localhost:3456", logger)
	dataChannel := listener.Start()
	sinkServer := sink.NewSinkServer(dataChannel, logger, "localhost:8081", "/tail/", "http://localhost:9876", SuccessfulAuthorizer, 30*time.Second)
	go sinkServer.Start()
	time.Sleep(1 * time.Millisecond)

	receivedChan := make(chan []byte)
	ws, _ := AddWSSink(t, receivedChan, "8081", "/tail/spaces/mySpace/apps/myApp?authorization=bearer%20correctAuthorizationToken")
	defer ws.Close()
	time.Sleep(50 * time.Millisecond)

	connection, err := net.Dial("udp", "localhost:3456")

	expectedMessageString := "Some Data"
	expectedMessage := MarshalledLogMessage(t, expectedMessageString, "mySpace", "myApp")

	_, err = connection.Write(expectedMessage)
	assert.NoError(t, err)

	AssertProtoBufferMessageEquals(t, expectedMessageString, <-receivedChan)
}
