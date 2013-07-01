package loggregator

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"code.google.com/p/go.net/websocket"
	"runtime"
)

var sink *cfSink
var dataReadChannel chan []byte

func init() {
	dataReadChannel = make(chan []byte)

	sink = NewCfSink(dataReadChannel, logger)
	go sink.Start()
}

func TestThatItSends(t *testing.T) {
	receivedChan := make(chan []byte, 2)

	expectedData := "Some Data"
	otherData := "More stuff"

	ws, err := websocket.Dial("ws://localhost:8080/tail", "string", "http://localhost")
	assert.NoError(t, err)
	defer ws.Close()

	go func() {
		for  {
			var data []byte
			err := websocket.Message.Receive(ws, &data)
			if err != nil {
				break
			}
			receivedChan <- data
		}
	}()

	dataReadChannel <- []byte(expectedData)
	dataReadChannel <- []byte(otherData)

	received := <-receivedChan
	assert.Equal(t, expectedData, string(received))

	receivedAgain := <-receivedChan
	assert.Equal(t, otherData, string(receivedAgain))
}

func TestThatSinkRelayStopsWhenClosed(t *testing.T) {
	t.Skip("Not properly closing we think... will revisit")

	expectedData := "Some Data"

	ws, err := websocket.Dial("ws://localhost:8080/tail", "string", "http://localhost")
	assert.NoError(t, err)

	err = ws.Close()
	assert.NoError(t, err)

	dataReadChannel <- []byte(expectedData)

	runtime.Gosched()

	assert.Equal(t, 1, len(dataReadChannel))
}
