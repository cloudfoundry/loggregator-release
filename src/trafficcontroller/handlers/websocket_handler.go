package handlers

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/gorilla/websocket"
	"net/http"
	"servertools"
	"time"
)

type websocketHandler struct {
	messages  <-chan []byte
	logger    *gosteno.Logger
	keepAlive time.Duration
}

func NewWebsocketHandler(m <-chan []byte, logger *gosteno.Logger, keepAlive time.Duration) *websocketHandler {
	return &websocketHandler{messages: m, logger: logger, keepAlive: keepAlive}
}

func (h *websocketHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Upgrade(rw, r, nil, 0, 0)
	if err != nil {
		http.Error(rw, "Not a websocket handshake", http.StatusBadRequest)
		return
	}
	defer ws.Close()
	defer ws.WriteControl(websocket.CloseMessage, []byte{}, time.Time{})
	keepAliveExpired := make(chan struct{})

	// TODO: remove this loop (but keep ws.ReadMessage()) once we retire support in the cli for old style keep alives
	go func() {
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	go func() {
		servertools.NewKeepAlive(ws, h.keepAlive).Run()
		close(keepAliveExpired)
	}()

	for {
		select {
		case <-keepAliveExpired:
			return
		case message, ok := <-h.messages:
			if !ok {
				return
			}
			err = ws.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				return
			}
		}
	}
}
