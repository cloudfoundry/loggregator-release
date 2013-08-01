package agentlistener

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"instrumentor"
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

	expectedDumpedData := []instrumentor.PropVal{
		instrumentor.PropVal{"CurrentBufferCount", "1"},
		instrumentor.PropVal{"ReceivedMessageCount", "2"},
		instrumentor.PropVal{"ReceivedByteCount", "19"},
	}
	assert.Equal(t, expectedDumpedData, listener.DumpData())

	receivedAgain := <-dataChannel
	assert.Equal(t, otherData, string(receivedAgain))

	expectedDumpedData = []instrumentor.PropVal{
		instrumentor.PropVal{"CurrentBufferCount", "0"},
		instrumentor.PropVal{"ReceivedMessageCount", "2"},
		instrumentor.PropVal{"ReceivedByteCount", "19"},
	}
	assert.Equal(t, expectedDumpedData, listener.DumpData())
}
