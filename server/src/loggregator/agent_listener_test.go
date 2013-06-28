package loggregator

import (
	"testing"
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"net"
)

func TestThatItListens(t *testing.T) {
	logger := gosteno.NewLogger("TestLogger")

	listener := newAgentListener("localhost:3456", logger)
	dataChannel := listener.start()

	expectedData := "Some Data"
	otherData := "More stuff"

	connection, err := net.Dial("udp", "localhost:3456")

	_, err = connection.Write([]byte(expectedData))
    assert.NoError(t, err)

	_, err = connection.Write([]byte(otherData))
    assert.NoError(t, err)

	received := <- dataChannel
	assert.Equal(t, expectedData, string(received))

	receivedAgain := <- dataChannel
	assert.Equal(t, otherData, string(receivedAgain))
}
