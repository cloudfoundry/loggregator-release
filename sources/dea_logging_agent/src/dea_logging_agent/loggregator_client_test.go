package dea_logging_agent

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestSend(t *testing.T) {
	expectedOutput := []byte("Important Testmessage")
	loggregatorClient := &TcpLoggregatorClient{}

	tcpListener, err := net.Listen("tcp", config.LoggregatorAddress)
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
