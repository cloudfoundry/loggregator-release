package loggregator

import (
	"github.com/cloudfoundry/gosteno"
	"net/http"
	"code.google.com/p/go.net/websocket"
)

type cfSink struct {
	*gosteno.Logger
	dataChannel chan []byte
	listenHost string
}

func NewCfSink(givenChannel chan []byte, logger *gosteno.Logger, listenHost string) (*cfSink) {
	return &cfSink{logger, givenChannel, listenHost}
}

func (cfSink *cfSink) sinkRelayHandler(ws *websocket.Conn) {
	clientAddress := ws.RemoteAddr()
	for {
		cfSink.Infof("Tail client %s is waiting for data\n", clientAddress)
		data := <-cfSink.dataChannel
		cfSink.Debugf("Tail client %s got %d bytes\n", clientAddress, len(data))
		err := websocket.Message.Send(ws, data)
		if (err != nil) {
			cfSink.Infof("Tail client %s must have gone away %s\n", clientAddress, err)
			break;
		}
	}
}

func (cfSink *cfSink) Start() {
	http.Handle("/tail", websocket.Handler(cfSink.sinkRelayHandler))
	cfSink.Infof("Listening on port %s", cfSink.listenHost)
	err := http.ListenAndServe(cfSink.listenHost, nil)
	if err != nil {
		panic(err)
	}
}
