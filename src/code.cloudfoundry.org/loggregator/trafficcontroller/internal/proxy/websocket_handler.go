package proxy

import (
	"log"
	"net/http"
	"time"

	"code.cloudfoundry.org/loggregator/metricemitter"

	"github.com/gorilla/websocket"
)

type websocketHandler struct {
	messages     <-chan []byte
	keepAlive    time.Duration
	egressMetric *metricemitter.Counter
}

func NewWebsocketHandler(
	m <-chan []byte,
	keepAlive time.Duration,
	egressMetric *metricemitter.Counter,
) *websocketHandler {
	return &websocketHandler{
		messages:     m,
		keepAlive:    keepAlive,
		egressMetric: egressMetric,
	}
}

func (h *websocketHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		log.Printf("websocket handler: Not a websocket handshake: %s", err)
		return
	}
	defer ws.Close()

	closeCode, closeMessage := h.runWebsocketUntilClosed(ws)
	ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(closeCode, closeMessage), time.Time{})
}

func (h *websocketHandler) runWebsocketUntilClosed(ws *websocket.Conn) (closeCode int, closeMessage string) {
	keepAliveExpired := make(chan struct{})
	clientWentAway := make(chan struct{})

	go func() {
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				close(clientWentAway)
				return
			}
		}
	}()

	go func() {
		NewKeepAlive(ws, h.keepAlive).Run()
		close(keepAliveExpired)
	}()

	closeCode = websocket.CloseNormalClosure
	closeMessage = ""
	for {
		select {
		case <-clientWentAway:
			return
		case <-keepAliveExpired:
			closeCode = websocket.ClosePolicyViolation
			closeMessage = "Client did not respond to ping before keep-alive timeout expired."
			return
		case message, ok := <-h.messages:
			if !ok {
				return
			}
			err := ws.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				return
			}
			h.egressMetric.Increment(1)
		}
	}
}
