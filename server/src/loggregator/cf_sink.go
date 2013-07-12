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
	logger           *gosteno.Logger
	dataChannel      chan []byte
	listenHost       string
	listenPath       string
	apiHost          string
	listenerChannels *groupedChannels
	authorize        LogAccessAuthorizer
}

func NewCfSinkServer(givenChannel chan []byte, logger *gosteno.Logger, listenHost string, listenPath string, apiHost string, authorize LogAccessAuthorizer) *cfSinkServer {
	listeners := newGroupedChannels()
	return &cfSinkServer{logger, givenChannel, listenHost, listenPath, apiHost, listeners, authorize}
}

func (cfSinkServer *cfSinkServer) sinkRelayHandler(ws *websocket.Conn) {
	extractAppIdAndAuthTokenFromUrl := func(u *url.URL) (string, string) {
		authorization := ""
		queryValues := u.Query()
		if len(queryValues["authorization"]) == 1 {
			authorization = queryValues["authorization"][0]
		}

		appId := ""
		re := regexp.MustCompile("^" + cfSinkServer.listenPath + "(.+)$")
		result := re.FindStringSubmatch(u.Path)
		if len(result) == 2 {
			appId = result[1]
		}

		return appId, authorization
	}

	clientAddress := ws.RemoteAddr()

	appId, authToken := extractAppIdAndAuthTokenFromUrl(ws.Request().URL)

	if appId == "" {
		cfSinkServer.logger.Warnf("Did not accept sink connection without appId: %s", clientAddress)
		return
	}
	if authToken == "" {
		cfSinkServer.logger.Warnf("Did not accept sink connection without authorization: %s", clientAddress)
		return
	}
	if !cfSinkServer.authorize(cfSinkServer.apiHost, authToken, appId, cfSinkServer.logger) {
		cfSinkServer.logger.Warnf("User not authorized to access app: %s", appId)
		return
	}

	listenerChannel := make(chan []byte)
	cfSinkServer.listenerChannels.add(appId, listenerChannel)
	defer cfSinkServer.listenerChannels.delete(appId, listenerChannel)

	for {
		cfSinkServer.logger.Infof("Tail client %s is waiting for data\n", clientAddress)
		data := <-listenerChannel
		cfSinkServer.logger.Debugf("Tail client %s got %d bytes\n", clientAddress, len(data))
		err := websocket.Message.Send(ws, data)
		if err != nil {
			cfSinkServer.logger.Infof("Tail client %s must have gone away %s\n", clientAddress, err)
			break
		}
	}
}

func (cfSinkServer *cfSinkServer) relayMessagesToAllSinks() {
	extractReceivedAppId := func(data []byte) string {
		receivedMessage := &logMessage.LogMessage{}
		err := proto.Unmarshal(data, receivedMessage)
		if err != nil {
			cfSinkServer.logger.Debugf("Log message could not be unmarshaled. Dropping it... Error: %v. Data: %v", err, data)
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
	cfSinkServer.logger.Infof("Listening on port %s", cfSinkServer.listenHost)
	err := http.ListenAndServe(cfSinkServer.listenHost, nil)
	if err != nil {
		panic(err)
	}
}
