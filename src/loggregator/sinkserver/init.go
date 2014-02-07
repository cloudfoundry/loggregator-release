package sinkserver

import (
	"encoding/binary"
	//	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	//	"loggregator/iprange"
	"github.com/gorilla/websocket"
	"net/http"
	"testing"
	"time"
)

var sinkManager *SinkManager

var TestMessageRouter *MessageRouter
var TestWebsocketServer *WebsocketServer
var dataReadChannel chan *logmessage.Message

var blacklistTestMessageRouter *MessageRouter
var blackListTestWebsocketServer *WebsocketServer
var blackListDataReadChannel chan *logmessage.Message

const (
	SERVER_PORT           = "8081"
	BLACKLIST_SERVER_PORT = "8082"
)

const SECRET = "secret"

func init() {
	dataReadChannel = make(chan *logmessage.Message)

	logger := loggertesthelper.Logger()

	sinkManager = NewSinkManager(1024, false, nil, logger)
	go sinkManager.Start()

	TestMessageRouter = NewMessageRouter(dataReadChannel, sinkManager, logger)
	go TestMessageRouter.Start()

	apiEndpoint := "localhost:" + SERVER_PORT
	TestWebsocketServer = NewWebsocketServer(apiEndpoint, sinkManager, 10*time.Millisecond, 100, loggertesthelper.Logger())
	go TestWebsocketServer.Start()

	blackListDataReadChannel = make(chan *logmessage.Message)
	blacklistSinkManager := NewSinkManager(1024, false, []iprange.IPRange{iprange.IPRange{Start: "127.0.0.0", End: "127.0.0.2"}}, logger)
	go blacklistSinkManager.Start()

	blacklistTestMessageRouter := NewMessageRouter(blackListDataReadChannel, blacklistSinkManager, logger)
	go blacklistTestMessageRouter.Start()

	blacklistApiEndpoint := "localhost:" + BLACKLIST_SERVER_PORT
	blackListTestWebsocketServer = NewWebsocketServer(blacklistApiEndpoint, blacklistSinkManager, 10*time.Millisecond, 100, loggertesthelper.Logger())
	go blackListTestWebsocketServer.Start()

	time.Sleep(2 * time.Millisecond)
}

func WaitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}

func AssertConnectionFails(t *testing.T, port string, path string, expectedErrorCode uint16) {
	ws, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})
	assert.NoError(t, err)
	_, data, err := ws.ReadMessage()
	errorCode := binary.BigEndian.Uint16(data)
	assert.Equal(t, expectedErrorCode, errorCode)
	assert.Equal(t, "EOF", err.Error())
}
