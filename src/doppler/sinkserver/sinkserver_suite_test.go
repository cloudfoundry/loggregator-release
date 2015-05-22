package sinkserver_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSinkserver(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Sinkserver Suite")
}

func AddWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool, <-chan bool) {
	dontKeepAliveChan := make(chan bool, 1)
	connectionDroppedChannel := make(chan bool, 1)

	ws, _, err := websocket.DefaultDialer.Dial("ws://localhost:"+port+path, http.Header{})
	Expect(err).NotTo(HaveOccurred())
	Expect(ws).NotTo(BeNil())

	go func() {
		for {
			_, data, err := ws.ReadMessage()

			if err != nil {
				connectionDroppedChannel <- true
				close(receivedChan)
				return
			}
			receivedChan <- data
		}

	}()

	go func() {
		for {
			err := ws.WriteMessage(websocket.BinaryMessage, []byte{42})
			if err != nil {
				break
			}
			select {
			case <-dontKeepAliveChan:
				return
			case <-time.After(4 * time.Millisecond):
				// keep-alive
			}
		}
	}()
	return ws, dontKeepAliveChan, connectionDroppedChannel
}
