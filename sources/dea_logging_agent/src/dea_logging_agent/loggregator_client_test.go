package dea_logging_agent

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestSend(t *testing.T) {
	expectedOutput := []byte("Important Testmessage")
	loggregatorClient := &UdpLoggregatorClient{}

	udpAddr, err := net.ResolveUDPAddr("udp", config.LoggregatorAddress)
	assert.NoError(t, err)

	udpListener, err := net.ListenUDP("udp", udpAddr)
	defer udpListener.Close()
	assert.NoError(t, err)

	loggregatorClient.Send(expectedOutput)


	buffer := make([]byte, bufferSize)
	readCount, _, err := udpListener.ReadFromUDP(buffer)
	assert.NoError(t, err)

	received := string(buffer[:readCount])
	assert.Equal(t, string(expectedOutput), received)
}
