package sinkserver_test

import (
	"net/http"
	"testing"

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

	if err != nil {
		Fail(err.Error())
	}
	return ws, dontKeepAliveChan, connectionDroppedChannel
}
