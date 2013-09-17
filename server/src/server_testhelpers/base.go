package server_testhelpers

import (
	"code.google.com/p/go.net/websocket"
	"github.com/cloudfoundry/gosteno"
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

func SuccessfulAuthorizer(authToken string, target string, l *gosteno.Logger) bool {
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
			case <-time.After(4 * time.Millisecond):
				// keep-alive
			}
		}
	}()
	return ws, dontKeepAliveChan, connectionDroppedChannel
}
