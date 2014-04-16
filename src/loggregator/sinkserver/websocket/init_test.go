package websocket_test

import (
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"loggregator/domain"
	"loggregator/sinkserver"
	"loggregator/sinkserver/blacklist"
	"loggregator/sinkserver/sinkmanager"
	"loggregator/sinkserver/websocket"
	"time"
)

var dataReadChannel chan *logmessage.Message
const SERVER_PORT              = "9081"

func init() {
	dataReadChannel = make(chan *logmessage.Message)

	logger := loggertesthelper.Logger()
	cfcomponent.Logger = logger

	newAppServiceChan := make(chan domain.AppService)
	deletedAppServiceChan := make(chan domain.AppService)

	emptyBlacklist := blacklist.New(nil)
	sinkManager, _ := sinkmanager.NewSinkManager(1024, false, emptyBlacklist, logger)
	go sinkManager.Start(newAppServiceChan, deletedAppServiceChan)

	testMessageRouter := sinkserver.NewMessageRouter(sinkManager, logger)
	go testMessageRouter.Start(dataReadChannel)

	apiEndpoint := "localhost:" + SERVER_PORT
	testWebsocketServer := websocket.NewWebsocketServer(apiEndpoint, sinkManager, 10*time.Millisecond, 100, logger)
	go testWebsocketServer.Start()

	time.Sleep(5 * time.Millisecond)
}

func WaitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}


