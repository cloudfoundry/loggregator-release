package websocketmessagereader

import (
	"fmt"
	"github.com/gorilla/websocket"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type WebsocketMessageReader struct {
	websocket       *websocket.Conn
	receivedCounter counter
}

type counter interface {
	IncrementValue()
}

func New(addr string, receivedCounter counter) WebsocketMessageReader {
	rand.Seed(time.Now().UnixNano())
	fullURL := "ws://" + addr + "/firehose/test" + strconv.Itoa(rand.Intn(100))

	ws, _, err := websocket.DefaultDialer.Dial(fullURL, http.Header{})
	if err != nil {
		panic(fmt.Sprintf("WebsocketMessageReader:New: %v", err))
	}

	return WebsocketMessageReader{
		websocket:       ws,
		receivedCounter: receivedCounter,
	}
}

func (wmr WebsocketMessageReader) ReadAndReturn() []byte {
	_, message, err := wmr.websocket.ReadMessage()
	if err != nil {
		panic(fmt.Sprintf("WebsocketMessageReader:Read: %v", err))
	}

	return message
}

func (wmr WebsocketMessageReader) Read() {
	wmr.ReadAndReturn()
	wmr.receivedCounter.IncrementValue()
}

func (wmr WebsocketMessageReader) Close() {
	wmr.websocket.Close()
}
