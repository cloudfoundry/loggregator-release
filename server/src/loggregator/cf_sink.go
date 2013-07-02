package loggregator

import (
	"github.com/cloudfoundry/gosteno"
	"net/http"
	"code.google.com/p/go.net/websocket"
)

type cfSink struct {
	dataChannel chan []byte
}

func NewCfSink(givenChannel chan []byte, logger *gosteno.Logger) (*cfSink) {
	return &cfSink{givenChannel}
}

func (cfSink *cfSink) sinkRelayHandler(ws *websocket.Conn) {
	clientAddress := ws.RemoteAddr()
	for {
		logger.Infof("Tail client %s is waiting for data\n", clientAddress)
		data := <-cfSink.dataChannel
		logger.Debugf("Tail client %s got %d bytes\n", clientAddress, len(data))
		err := websocket.Message.Send(ws, data)
		if (err != nil) {
			logger.Infof("Tail client %s must have gone away %s\n", clientAddress, err)
			break;
		}
	}
}

func (cfSink *cfSink) Start() {
	http.Handle("/tail", websocket.Handler(cfSink.sinkRelayHandler))
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
