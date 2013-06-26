package dea_logging_agent

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"os"
	"bytes"
	"net"
)

func TestIdentifier(t *testing.T) {
	instance := Instance{
		ApplicationId: "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		WardenJobId: "272",
		WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}

	assert.Equal(t, "/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272", instance.Identifier())
}

type MockSinkServer struct{
	received []byte
}

func (m *MockSinkServer) Send(data []byte) {
	m.received = data
}

func TestThatWeListenToTheUnixSockets(t *testing.T) {
	instance := &Instance{
		ApplicationId: "1234",
		WardenJobId: "56",
		WardenContainerPath: "/tmp" }
	socketPath := filepath.Join(instance.Identifier(), "stdout.sock")
	os.Remove(socketPath)
	expectedOutput := bytes.NewBufferString("Some Output\n").Bytes()

	stdoutListener, err := net.Listen("unix", socketPath)
	assert.NoError(t, err)
	go func() {
		connection, err := stdoutListener.Accept()
		defer connection.Close()
		assert.NoError(t, err)

		_, err = connection.Write(expectedOutput)
		assert.NoError(t, err)

	}()

	mockSinkServer := new(MockSinkServer)

	instance.StartListening(mockSinkServer)
	instance.StopListening()
	<-instance.listenerControlChannel

	assert.Equal(t, expectedOutput, mockSinkServer.received)
}
