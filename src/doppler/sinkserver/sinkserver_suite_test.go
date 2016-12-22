package sinkserver_test

import (
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSinkserver(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)

	RunSpecs(t, "Sinkserver Suite")
}

func AddWSSink(receivedChan chan []byte, port string, path string) (*websocket.Conn, chan bool, <-chan bool) {
	dontKeepAliveChan := make(chan bool, 1)
	connectionDroppedChannel := make(chan bool, 1)

	var ws *websocket.Conn
	var err error
	Eventually(func() error {
		ws, _, err = websocket.DefaultDialer.Dial("ws://127.0.0.1:"+port+path, http.Header{})
		return err
	}).Should(Succeed())
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
			case <-time.After(10 * time.Millisecond):
				// keep-alive
			}
		}
	}()
	return ws, dontKeepAliveChan, connectionDroppedChannel
}
