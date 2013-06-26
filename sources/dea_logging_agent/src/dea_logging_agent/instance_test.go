package dea_logging_agent

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestIdentifier(t *testing.T) {
	instance := Instance{
		ApplicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		WardenJobId:         "272",
		WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}

	assert.Equal(t, "/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272", instance.Identifier())
}

type MockLoggregatorClient struct {
	received chan []byte
}

func (m *MockLoggregatorClient) Send(data []byte) {
	m.received <- data
}

func TestThatWeListenToTheUnixSockets(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	instance := &Instance{
		ApplicationId:       "1234",
		WardenJobId:         "56",
		WardenContainerPath: tmpdir}
	os.MkdirAll(instance.Identifier(), 0777)

	socketPath := filepath.Join(instance.Identifier(), "stdout.sock")
	os.Remove(socketPath)
	expectedOutput := bytes.NewBufferString("Some Output\n").Bytes()
	moreExpectedOutput := bytes.NewBufferString("toally different\n").Bytes()

	stdoutListener, err := net.Listen("unix", socketPath)
	assert.NoError(t, err)

	mockLoggregatorClient := new(MockLoggregatorClient)

	mockLoggregatorClient.received = make(chan []byte)
	instance.StartListening(mockLoggregatorClient)

	connection, err := stdoutListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write(expectedOutput)
	assert.NoError(t, err)

	data := <- mockLoggregatorClient.received
	assert.Equal(t, expectedOutput, data)

	_, err = connection.Write(moreExpectedOutput)
	assert.NoError(t, err)

	data = <- mockLoggregatorClient.received
	assert.Equal(t, moreExpectedOutput, data)

	instance.StopListening()
	<-instance.listenerControlChannel
}
