package loggregator

import (
	"code.google.com/p/go.net/websocket"
	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry/gosteno"
	"logMessage"
	"net/http"
	"net/url"
	"regexp"
)

type cfSinkServer struct {
	*gosteno.Logger
	dataChannel      chan []byte
	listenHost       string
	listenPath       string
	listenerChannels *groupedChannels
}

func NewCfSinkServer(givenChannel chan []byte, logger *gosteno.Logger, listenHost string, listenPath string) *cfSinkServer {
	listeners := newGroupedChannels()
	return &cfSinkServer{logger, givenChannel, listenHost, listenPath, listeners}
}

func (cfSinkServer *cfSinkServer) sinkRelayHandler(ws *websocket.Conn) {
	extractAppIdFromUrl := func(url *url.URL) string {
		re := regexp.MustCompile("^" + cfSinkServer.listenPath + "(.+)$")
		result := re.FindStringSubmatch(url.String())
		return result[len(result)-1]
	}

	listenerChannel := make(chan []byte)
	appId := extractAppIdFromUrl(ws.Request().URL)
	cfSinkServer.listenerChannels.add(appId, listenerChannel)
	defer cfSinkServer.listenerChannels.delete(appId, listenerChannel)

	clientAddress := ws.RemoteAddr()
	for {
		cfSinkServer.Infof("Tail client %s is waiting for data\n", clientAddress)
		data := <-listenerChannel
		cfSinkServer.Debugf("Tail client %s got %d bytes\n", clientAddress, len(data))
		err := websocket.Message.Send(ws, data)
		if err != nil {
			cfSinkServer.Infof("Tail client %s must have gone away %s\n", clientAddress, err)
			break
		}
	}
}

func (cfSinkServer *cfSinkServer) relayMessagesToAllSinks() {
	extractReceivedAppId := func(data []byte) string {
		receivedMessage := &logMessage.LogMessage{}
		err := proto.Unmarshal(data, receivedMessage)
		if err != nil {
			cfSinkServer.Debugf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, data)
			return ""
		}
		return *receivedMessage.AppId
	}

	for {
		data := <-cfSinkServer.dataChannel
		receivedAppId := extractReceivedAppId(data)
		for _, listenerChannel := range cfSinkServer.listenerChannels.get(receivedAppId) {
			listenerChannel <- data
		}
	}
}

func (cfSinkServer *cfSinkServer) Start() {
	go cfSinkServer.relayMessagesToAllSinks()
	http.Handle(cfSinkServer.listenPath, websocket.Handler(cfSinkServer.sinkRelayHandler))
	cfSinkServer.Infof("Listening on port %s", cfSinkServer.listenHost)
	err := http.ListenAndServe(cfSinkServer.listenHost, nil)
	if err != nil {
		panic(err)
	}
}
