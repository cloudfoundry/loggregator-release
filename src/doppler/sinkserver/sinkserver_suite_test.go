package sinkserver_test

import (
	"doppler/envelopewrapper"
	"doppler/sinkserver"
	"doppler/sinkserver/blacklist"
	"doppler/sinkserver/sinkmanager"
	"doppler/sinkserver/websocketserver"
	"github.com/cloudfoundry/loggregatorlib/appservice"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/gorilla/websocket"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSinkserver(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Sinkserver Suite")
}

var (
	sinkManager         *sinkmanager.SinkManager
	TestMessageRouter   *sinkserver.MessageRouter
	TestWebsocketServer *websocketserver.WebsocketServer
	dataReadChannel     chan *envelopewrapper.WrappedEnvelope
)

const (
	SERVER_PORT = "9081"
)

var _ = BeforeSuite(func() {
	dataReadChannel = make(chan *envelopewrapper.WrappedEnvelope)

	logger := loggertesthelper.Logger()
	cfcomponent.Logger = logger

	newAppServiceChan := make(chan appservice.AppService)
	deletedAppServiceChan := make(chan appservice.AppService)

	emptyBlacklist := blacklist.New(nil)
	sinkManager, _ = sinkmanager.NewSinkManager(1024, false, emptyBlacklist, logger, "dropsonde-origin")
	go sinkManager.Start(newAppServiceChan, deletedAppServiceChan)

	TestMessageRouter = sinkserver.NewMessageRouter(sinkManager, logger)
	go TestMessageRouter.Start(dataReadChannel)

	apiEndpoint := "localhost:" + SERVER_PORT
	TestWebsocketServer = websocketserver.New(apiEndpoint, sinkManager, 10*time.Second, 100, loggertesthelper.Logger())
	go TestWebsocketServer.Start()

	time.Sleep(5 * time.Millisecond)
})

func AddWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool, <-chan bool) {

	dontKeepAliveChan := make(chan bool, 1)
	connectionDroppedChannel := make(chan bool, 1)
	ws, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})

	if err != nil {
		Fail(err.Error())
	}
	return ws, dontKeepAliveChan, connectionDroppedChannel
}

func WaitForWebsocketRegistration() {
	time.Sleep(50 * time.Millisecond)
}
