package loggregator

import (
	"code.google.com/p/go.net/websocket"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func addWSSink(t *testing.T, receivedChan chan []byte, port string, path string) (ws *websocket.Conn) {
	ws, err := websocket.Dial("ws://localhost:"+port+path, "string", "http://localhost")
	assert.NoError(t, err)

	go func() {
		for {
			var data []byte
			err := websocket.Message.Receive(ws, &data)
			if err != nil {
				break
			}
			receivedChan <- data
		}
	}()
	return ws
}

func TestThatItSends(t *testing.T) {
	dataReadChannel := make(chan []byte)

	sink := NewCfSinkServer(dataReadChannel, gosteno.NewLogger("TestLogger"), "localhost:8081", "/tail")
	go sink.Start()
	time.Sleep(1 * time.Millisecond)

	receivedChan := make(chan []byte, 2)

	expectedData := "Some Data"
	otherData := "More stuff"

	ws := addWSSink(t, receivedChan, "8081", "/tail")
	defer ws.Close()

	dataReadChannel <- []byte(expectedData)
	dataReadChannel <- []byte(otherData)

	received := <-receivedChan
	assert.Equal(t, expectedData, string(received))

	receivedAgain := <-receivedChan
	assert.Equal(t, otherData, string(receivedAgain))
}

func TestThatItSendsAllDataToAllSinks(t *testing.T) {
	dataReadChannel := make(chan []byte)

	sink := NewCfSinkServer(dataReadChannel, gosteno.NewLogger("TestLogger"), "localhost:8082", "/tail2")
	go sink.Start()
	time.Sleep(1 * time.Millisecond)

	client1ReceivedChan := make(chan []byte)
	client2ReceivedChan := make(chan []byte)

	expectedData := "Some Data"

	wsClient1 := addWSSink(t, client1ReceivedChan, "8082", "/tail2")
	defer wsClient1.Close()

	wsClient2 := addWSSink(t, client2ReceivedChan, "8082", "/tail2")
	defer wsClient2.Close()

	dataReadChannel <- []byte(expectedData)

	client1Received := <-client1ReceivedChan
	assert.Equal(t, expectedData, string(client1Received))

	client2Received := <-client2ReceivedChan
	assert.Equal(t, expectedData, string(client2Received))
}
