package websocketmessagereader

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

type WebsocketMessageReader struct {
	websocket       *websocket.Conn
	receivedCounter counter
}

type counter interface {
	IncrementValue()
}

func New(addr string, receivedCounter counter) *WebsocketMessageReader {
	rand.Seed(time.Now().UnixNano())
	fullURL := "ws://" + addr + "/firehose/test" + strconv.Itoa(rand.Intn(100))

	ws, _, err := websocket.DefaultDialer.Dial(fullURL, http.Header{})
	if err != nil {
		panic(fmt.Sprintf("WebsocketMessageReader:New: %v", err))
	}

	return &WebsocketMessageReader{
		websocket:       ws,
		receivedCounter: receivedCounter,
	}
}

func (wmr *WebsocketMessageReader) Read() {
	_, _, err := wmr.websocket.ReadMessage()
	if err == nil {
		wmr.receivedCounter.IncrementValue()
	}
}

func (wmr *WebsocketMessageReader) Close() {
	wmr.websocket.Close()
}
