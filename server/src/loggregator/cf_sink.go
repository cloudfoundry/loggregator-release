package loggregator

import (
	"code.google.com/p/go.net/websocket"
	"github.com/cloudfoundry/gosteno"
	"net/http"
)

type cfSinkServer struct {
	*gosteno.Logger
	dataChannel      chan []byte
	listenHost       string
	listenPath       string
	listenerChannels map[chan []byte]bool
}

func NewCfSinkServer(givenChannel chan []byte, logger *gosteno.Logger, listenHost string, listenPath string) *cfSinkServer {
	listeners := make(map[chan []byte]bool)
	return &cfSinkServer{logger, givenChannel, listenHost, listenPath, listeners}
}

func (cfSinkServer *cfSinkServer) sinkRelayHandler(ws *websocket.Conn) {
	listenerChannel := make(chan []byte)
	cfSinkServer.listenerChannels[listenerChannel] = true

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

	delete(cfSinkServer.listenerChannels, listenerChannel)
}

func (cfSinkServer *cfSinkServer) relayMessagesToAllSinks() {
	for {
		data := <-cfSinkServer.dataChannel
		for listenerChannel, _ := range cfSinkServer.listenerChannels {
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
