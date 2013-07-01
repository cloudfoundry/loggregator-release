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
	for {
		logger.Infof("Looking for data")
		data := <-cfSink.dataChannel
		logger.Infof("Got: %s\n", data)
		err := websocket.Message.Send(ws, data)
		if (err != nil) {
			logger.Warnf("Error %s\n", err)
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
