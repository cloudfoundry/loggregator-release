package agentlistener

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

var listener *agentListener
var dataChannel chan []byte

func init() {
	listener = NewAgentListener("localhost:3456", gosteno.NewLogger("TestLogger"))
	dataChannel = listener.Start()
}

func TestThatItListens(t *testing.T) {

	expectedData := "Some Data"
	otherData := "More stuff"

	connection, err := net.Dial("udp", "localhost:3456")

	_, err = connection.Write([]byte(expectedData))
	assert.NoError(t, err)

	_, err = connection.Write([]byte(otherData))
	assert.NoError(t, err)

	received := <-dataChannel
	assert.Equal(t, expectedData, string(received))

	receivedAgain := <-dataChannel
	assert.Equal(t, otherData, string(receivedAgain))
}
