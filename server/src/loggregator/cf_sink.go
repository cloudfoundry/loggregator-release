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
	apiHost          string
	listenerChannels *groupedChannels
}

func NewCfSinkServer(givenChannel chan []byte, logger *gosteno.Logger, listenHost string, listenPath string, apiHost string) *cfSinkServer {
	listeners := newGroupedChannels()
	return &cfSinkServer{logger, givenChannel, listenHost, listenPath, apiHost, listeners}
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

	authorizedToListenToLogs := func(authToken, appId string) bool {
		client := &http.Client{}
		req, _ := http.NewRequest("GET", cfSinkServer.apiHost+"/v2/apps/"+appId, nil)
		req.Header.Set("Authorization", authToken)
		res, err := client.Do(req)

		if err != nil {
			cfSinkServer.Errorf("Did not accept sink connection because of connection issue with CC: %v", err)
			return false
		}
		if res.StatusCode != 200 {
			return false
		}

		return true
	}

	clientAddress := ws.RemoteAddr()

	appId, authToken := extractAppIdAndAuthTokenFromUrl(ws.Request().URL)

	if appId == "" {
		cfSinkServer.Warnf("Did not accept sink connection without appId: %s", clientAddress)
		return
	}
	if authToken == "" {
		cfSinkServer.Warnf("Did not accept sink connection without authorization: %s", clientAddress)
		return
	}
	if !authorizedToListenToLogs(authToken, appId) {
		cfSinkServer.Warnf("User not authorized to access app: %s", appId)
		return
	}

	listenerChannel := make(chan []byte)
	cfSinkServer.listenerChannels.add(appId, listenerChannel)
	defer cfSinkServer.listenerChannels.delete(appId, listenerChannel)

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
