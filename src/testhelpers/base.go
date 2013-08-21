package testhelpers

import (
	"bytes"
	"code.google.com/p/go.net/websocket"
	"code.google.com/p/gogoprotobuf/proto"
	"encoding/binary"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/cloudfoundry/loggregatorlib/logtarget"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func StdOutLogger() *gosteno.Logger {
	return getLogger(true)
}

func Logger() *gosteno.Logger {
	return getLogger(false)
}

func getLogger(debug bool) *gosteno.Logger {
	if debug {
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

const (
	VALID_ORG_AUTHENTICATION_TOKEN   = "bearer correctOrgLevelAuthorizationToken"
	INVALID_AUTHENTICATION_TOKEN     = "incorrectAuthorizationToken"
	VALID_SPACE_AUTHENTICATION_TOKEN = "bearer correctSpaceLevelAuthorizationToken"
)

func SuccessfulAuthorizer(authToken string, target *logtarget.LogTarget, l *gosteno.Logger) bool {
	return authToken == VALID_SPACE_AUTHENTICATION_TOKEN || authToken == VALID_ORG_AUTHENTICATION_TOKEN
}

func AddWSSink(t *testing.T, receivedChan chan []byte, port string, path string, authToken string) (*websocket.Conn, chan bool, <-chan bool) {
	dontKeepAliveChan := make(chan bool)
	connectionDroppedChannel := make(chan bool)

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
				connectionDroppedChannel <- true
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
			case <-time.After(44 * time.Millisecond):
				// keep-alive
			}
		}
	}()
	return ws, dontKeepAliveChan, connectionDroppedChannel
}

func MarshalledLogMessage(t *testing.T, messageString string, appId string) []byte {
	currentTime := time.Now()

	messageType := logmessage.LogMessage_OUT
	sourceType := logmessage.LogMessage_DEA
	protoMessage := &logmessage.LogMessage{
		Message:     []byte(messageString),
		AppId:       proto.String(appId),
		MessageType: &messageType,
		SourceType:  &sourceType,
		Timestamp:   proto.Int64(currentTime.UnixNano()),
	}

	message, err := proto.Marshal(protoMessage)
	assert.NoError(t, err)

	return message
}

func AssertProtoBufferMessageEquals(t *testing.T, expectedMessage string, actual []byte) {
	receivedMessage := &logmessage.LogMessage{}

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
