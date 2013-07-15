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
	extractAppIdAndAuthTokenFromUrl := func(u *url.URL) (string, string, string) {
		authorization := ""
		queryValues := u.Query()
		if len(queryValues["authorization"]) == 1 {
			authorization = queryValues["authorization"][0]
		}
		appId := ""
		spaceId := ""
		re := regexp.MustCompile("^" + cfSinkServer.listenPath + "spaces/([^/]+)(?:/apps/([^/]+))?$")
		result := re.FindStringSubmatch(u.Path)

		switch len(result) {
		case 2:
			spaceId = result[1]
		case 3:
			spaceId = result[1]
			appId = result[2]
		}

		return spaceId, appId, authorization
	}

	clientAddress := ws.RemoteAddr()

	spaceId, appId, authToken := extractAppIdAndAuthTokenFromUrl(ws.Request().URL)

	if spaceId == "" {
		cfSinkServer.logger.Warnf("Did not accept sink connection without spaceId: %s", clientAddress)
		return
	}
	if appId == "" {
		cfSinkServer.logger.Warnf("Did not accept sink connection without appId: %s", clientAddress)
		return
	}
	if authToken == "" {
		cfSinkServer.logger.Warnf("Did not accept sink connection without authorization: %s", clientAddress)
		return
	}
	if !cfSinkServer.authorize(cfSinkServer.apiHost, authToken, spaceId, cfSinkServer.logger) {
		cfSinkServer.logger.Warnf("User not authorized to access space: %s", spaceId)
		return
	}

	listenerChannel := make(chan []byte)
	cfSinkServer.listenerChannels.add(listenerChannel, appId)
	defer cfSinkServer.listenerChannels.delete(listenerChannel, appId)

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
	extractReceivedSpaceAndAppId := func(data []byte) string {
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
		receivedAppId := extractReceivedSpaceAndAppId(data)
		//		for _, listenerChannel := range cfSinkServer.listenerChannels.get(receivedSpaceId, receivedAppId) {
		//			listenerChannel <- data
		//		}
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
