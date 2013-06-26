package dea_logging_agent

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"net"
)

func TestSend(t *testing.T) {
	expectedOutput := []byte("Important Testmessage")
	config := Config{loggregatorAddress: "localhost:9876"}
	loggregatorClient := &TcpLoggregatorClient{config: &config}

	tcpListener, err := net.Listen("tcp", config.loggregatorAddress)
	assert.NoError(t, err)

	loggregatorClient.Send(expectedOutput)

	connection, err := tcpListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	buffer := make([]byte, 128)
	readCount, err := connection.Read(buffer)
	assert.NoError(t, err)

	received := string(buffer[:readCount])
	assert.Equal(t, string(expectedOutput), received)
}
