package handlers

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

type websocketHandler struct {
	messages <-chan []byte
	logger   *gosteno.Logger
}

func NewWebsocketHandler(m <-chan []byte, logger *gosteno.Logger) *websocketHandler {
	return &websocketHandler{messages: m, logger: logger}
}

func (h *websocketHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Upgrade(rw, r, nil, 1024, 1024)
	if err != nil {
		http.Error(rw, "Not a websocket handshake", http.StatusBadRequest)
		return
	}
	defer ws.Close()
	defer ws.WriteControl(websocket.CloseMessage, []byte{}, time.Time{})

	for message := range h.messages {
		ws.WriteMessage(websocket.BinaryMessage, message)
	}
}

// TODO fix this - implement keepAlive so that connection doesn't die
//func (handler *handler) watchKeepAlive(servers []*websocket.Conn) {
//	for {
//		_, keepAlive, err := handler.clientWS.ReadMessage()
//
//		if err != nil {
//			handler.logger.Errorf("Output Proxy: Error reading from the client - %v", err)
//			for _, server := range servers {
//				server.WriteControl(websocket.CloseMessage, []byte{}, time.Time{})
//			}
//
//			return
//		}
//		handler.logger.Debugf("Output Proxy: Got message from client %v bytes", len(keepAlive))
//		for _, server := range servers {
//			server.WriteMessage(websocket.BinaryMessage, keepAlive)
//		}
//	}
//}
