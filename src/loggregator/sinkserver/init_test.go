package sinkserver_test

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/iprange"
	"loggregator/sinkserver"
	"loggregator/sinkserver/blacklist"
	"loggregator/sinkserver/sinkmanager"
	"loggregator/sinkserver/websocket"
	"time"
	"loggregator/domain"
)

var sinkManager *sinkmanager.SinkManager

var TestMessageRouter *sinkserver.MessageRouter
var TestWebsocketServer *websocket.WebsocketServer
var dataReadChannel chan *logmessage.Message

var blacklistTestMessageRouter *sinkserver.MessageRouter
var blackListTestWebsocketServer *websocket.WebsocketServer
var blackListDataReadChannel chan *logmessage.Message

const (
	SERVER_PORT              = "8081"
	BLACKLIST_SERVER_PORT    = "8082"
	FAST_TIMEOUT_SERVER_PORT = "8083"
)

const SECRET = "secret"

func init() {
	dataReadChannel = make(chan *logmessage.Message)

	logger := loggertesthelper.Logger()

	newAppServiceChan := make(chan domain.AppService)
	deletedAppServiceChan := make(chan domain.AppService)

	emptyBlacklist := blacklist.New(nil)
	sinkManager, _ = sinkmanager.NewSinkManager(1024, false, emptyBlacklist, logger)
	go sinkManager.Start(newAppServiceChan, deletedAppServiceChan)

	TestMessageRouter = sinkserver.NewMessageRouter(sinkManager, logger)
	go TestMessageRouter.Start(dataReadChannel)

	apiEndpoint := "localhost:" + SERVER_PORT
	TestWebsocketServer = websocket.NewWebsocketServer(apiEndpoint, sinkManager, 10*time.Second, 100, loggertesthelper.Logger())
	go TestWebsocketServer.Start()

	timeoutApiEndpoint := "localhost:" + FAST_TIMEOUT_SERVER_PORT
	FastTimeoutTestWebsocketServer := websocket.NewWebsocketServer(timeoutApiEndpoint, sinkManager, 10*time.Millisecond, 100, loggertesthelper.Logger())
	go FastTimeoutTestWebsocketServer.Start()

	blackListDataReadChannel = make(chan *logmessage.Message)
	localhostBlacklist := blacklist.New([]iprange.IPRange{iprange.IPRange{Start: "127.0.0.0", End: "127.0.0.2"}})
	blacklistSinkManager, _ := sinkmanager.NewSinkManager(1024, false, localhostBlacklist, logger)
	go blacklistSinkManager.Start(newAppServiceChan, deletedAppServiceChan)

	blacklistTestMessageRouter := sinkserver.NewMessageRouter(blacklistSinkManager, logger)
	go blacklistTestMessageRouter.Start(blackListDataReadChannel)

	blacklistApiEndpoint := "localhost:" + BLACKLIST_SERVER_PORT
	blackListTestWebsocketServer = websocket.NewWebsocketServer(blacklistApiEndpoint, blacklistSinkManager, 10*time.Second, 100, loggertesthelper.Logger())
	go blackListTestWebsocketServer.Start()

	time.Sleep(2 * time.Millisecond)
}

func WaitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}
