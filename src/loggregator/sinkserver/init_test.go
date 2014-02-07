package sinkserver_test

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"loggregator/iprange"
	"loggregator/sinkserver"
	"net/http"
	"testing"
	"time"
)

var sinkManager *sinkserver.SinkManager

var TestMessageRouter *sinkserver.MessageRouter
var TestWebsocketServer *sinkserver.WebsocketServer
var dataReadChannel chan *logmessage.Message

var blacklistTestMessageRouter *sinkserver.MessageRouter
var blackListTestWebsocketServer *sinkserver.WebsocketServer
var blackListDataReadChannel chan *logmessage.Message

const (
	SERVER_PORT           = "8081"
	BLACKLIST_SERVER_PORT = "8082"
)

const SECRET = "secret"

func init() {
	dataReadChannel = make(chan *logmessage.Message)

	logger := loggertesthelper.Logger()

	sinkManager = sinkserver.NewSinkManager(1024, false, nil, logger)
	go sinkManager.Start()

	TestMessageRouter = sinkserver.NewMessageRouter(dataReadChannel, sinkManager, logger)
	go TestMessageRouter.Start()

	apiEndpoint := "localhost:" + SERVER_PORT
	TestWebsocketServer = sinkserver.NewWebsocketServer(apiEndpoint, sinkManager, 10*time.Millisecond, 100, loggertesthelper.Logger())
	go TestWebsocketServer.Start()

	blackListDataReadChannel = make(chan *logmessage.Message)
	blacklistSinkManager := sinkserver.NewSinkManager(1024, false, []iprange.IPRange{iprange.IPRange{Start: "127.0.0.0", End: "127.0.0.2"}}, logger)
	go blacklistSinkManager.Start()

	blacklistTestMessageRouter := sinkserver.NewMessageRouter(blackListDataReadChannel, blacklistSinkManager, logger)
	go blacklistTestMessageRouter.Start()

	blacklistApiEndpoint := "localhost:" + BLACKLIST_SERVER_PORT
	blackListTestWebsocketServer = sinkserver.NewWebsocketServer(blacklistApiEndpoint, blacklistSinkManager, 10*time.Millisecond, 100, loggertesthelper.Logger())
	go blackListTestWebsocketServer.Start()

	time.Sleep(2 * time.Millisecond)
}

func WaitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}

func AssertConnectionFails(t *testing.T, port string, path string) {
	_, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})
	assert.Error(t, err)

}
