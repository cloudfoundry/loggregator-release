package testhelpers

import (
	"bytes"
	"code.google.com/p/go.net/websocket"
	"code.google.com/p/gogoprotobuf/proto"
	"encoding/binary"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"logMessage"
	"os"
	"strings"
	"testing"
	"time"
)

func Logger() *gosteno.Logger {
	if os.Getenv("LOG_TO_STDOUT") == "true" {
		level := gosteno.LOG_DEBUG

		loggingConfig := &gosteno.Config{
			Sinks:     make([]gosteno.Sink, 1),
			Level:     level,
			Codec:     gosteno.NewJsonCodec(),
			EnableLOC: true,
		}

		loggingConfig.Sinks[0] = gosteno.NewIOSink(os.Stdout)

		gosteno.Init(loggingConfig)
	}

	return gosteno.NewLogger("TestLogger")
}

func SuccessfulAuthorizer(authToken, spaceId, appId string, l *gosteno.Logger) bool {
	if authToken != "" {
		authString := strings.Split(authToken, " ")
		if len(authString) > 1 {
			return authString[1] == "correctAuthorizationToken"
		}
	}
	return false
}

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

func MarshalledLogMessage(t *testing.T, messageString string, spaceId string, appId string, organizationId string) []byte {
	currentTime := time.Now()

	messageType := logMessage.LogMessage_OUT
	sourceType := logMessage.LogMessage_DEA
	protoMessage := &logMessage.LogMessage{
		Message:        []byte(messageString),
		AppId:          proto.String(appId),
		OrganizationId: proto.String(organizationId),
		SpaceId:        proto.String(spaceId),
		MessageType:    &messageType,
		SourceType:     &sourceType,
		Timestamp:      proto.Int64(currentTime.UnixNano()),
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

func ParseDumpedMessages(b []byte) (messages [][]byte, err error) {
	buffer := bytes.NewBuffer(b)
	var length uint32
	for buffer.Len() > 0 {
		lengthBytes := bytes.NewBuffer(buffer.Next(4))
		err = binary.Read(lengthBytes, binary.BigEndian, &length)
		if err != nil {
			return
		}

		msg := buffer.Next(int(length))
		messages = append(messages, msg)
	}
	return
}
