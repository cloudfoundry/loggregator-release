package sinkserver

import (
	"code.google.com/p/go.net/websocket"
	"encoding/binary"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/stretchr/testify/assert"
	"loggregator/iprange"
	testhelpers "server_testhelpers"
	"testing"
	"time"
)

var sinkManager *SinkManager

var TestMessageRouter *messageRouter
var TestHttpServer *httpServer
var dataReadChannel chan []byte

var blacklistTestMessageRouter *messageRouter
var blackListTestHttpServer *httpServer
var blackListDataReadChannel chan []byte

const (
	SERVER_PORT           = "8081"
	BLACKLIST_SERVER_PORT = "8082"
)

const SECRET = "secret"

func init() {
	dataReadChannel = make(chan []byte, 20)

	logger := loggertesthelper.Logger()
	sinkManager = NewSinkManager(1024, false, nil, logger)
	go sinkManager.Start()

	TestMessageRouter = NewMessageRouter(sinkManager, 2048, logger)
	go TestMessageRouter.Start()

	TestHttpServer = NewHttpServer(TestMessageRouter, 10*time.Millisecond, testhelpers.UnmarshallerMaker(SECRET), 100, loggertesthelper.Logger())
	go TestHttpServer.Start(dataReadChannel, "localhost:"+SERVER_PORT)

	blackListDataReadChannel = make(chan []byte, 20)
	blacklistsinkManager := NewSinkManager(1024, false, []iprange.IPRange{iprange.IPRange{Start: "127.0.0.0", End: "127.0.0.2"}}, logger)
	go blacklistsinkManager.Start()

	blacklistTestMessageRouter := NewMessageRouter(blacklistsinkManager, 2048, logger)
	go blacklistTestMessageRouter.Start()

	blackListTestHttpServer = NewHttpServer(blacklistTestMessageRouter, 10*time.Millisecond, testhelpers.UnmarshallerMaker(SECRET), 100, loggertesthelper.Logger())
	go blackListTestHttpServer.Start(blackListDataReadChannel, "localhost:"+BLACKLIST_SERVER_PORT)
	time.Sleep(2 * time.Millisecond)
}

func WaitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}

func AssertConnectionFails(t *testing.T, port string, path string, expectedErrorCode uint16) {
	config, err := websocket.NewConfig("ws://localhost:"+port+path, "http://localhost")
	assert.NoError(t, err)

	ws, err := websocket.DialConfig(config)
	assert.NoError(t, err)
	data := make([]byte, 2)
	_, err = ws.Read(data)
	errorCode := binary.BigEndian.Uint16(data)
	assert.Equal(t, expectedErrorCode, errorCode)
	assert.Equal(t, "EOF", err.Error())
}
