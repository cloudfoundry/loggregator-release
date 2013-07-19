package deaagent

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"logMessage"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestIdentifier(t *testing.T) {
	instance := instance{
		applicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		wardenJobId:         272,
		wardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}

	assert.Equal(t, "/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272", instance.identifier())
}

type MockLoggregatorClient struct {
	received chan *[]byte
}

func (m *MockLoggregatorClient) Send(data []byte) {
	m.received <- &data
}

func TestThatWeListenToStdOutUnixSocket(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	instance := &instance{
		applicationId:       "1234",
		spaceId:             "5678",
		wardenJobId:         56,
		wardenContainerPath: tmpdir,
		index:               3}
	os.MkdirAll(instance.identifier(), 0777)

	stdoutSocketPath := filepath.Join(instance.identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(instance.identifier(), "stderr.sock")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	stdoutListener, err := net.Listen("unix", stdoutSocketPath)
	defer stdoutListener.Close()
	assert.NoError(t, err)
	stderrListener, err := net.Listen("unix", stderrSocketPath)
	defer stderrListener.Close()
	assert.NoError(t, err)

	expectedMessage := "Some Output\n"
	secondLogMessage := "toally different\n"

	mockLoggregatorClient := new(MockLoggregatorClient)

	mockLoggregatorClient.received = make(chan *[]byte)
	instance.startListening(mockLoggregatorClient, logger())

	connection, err := stdoutListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(expectedMessage))
	assert.NoError(t, err)

	receivedMessage := getBackendMessage(t, <-mockLoggregatorClient.received)

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, "5678", receivedMessage.GetSpaceId())
	assert.Equal(t, logMessage.LogMessage_DEA, receivedMessage.GetSourceType())
	assert.Equal(t, logMessage.LogMessage_OUT, receivedMessage.GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))

	_, err = connection.Write([]byte(secondLogMessage))
	assert.NoError(t, err)

	receivedMessage = getBackendMessage(t, <-mockLoggregatorClient.received)

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, logMessage.LogMessage_DEA, receivedMessage.GetSourceType())
	assert.Equal(t, logMessage.LogMessage_OUT, receivedMessage.GetMessageType())
	assert.Equal(t, secondLogMessage, string(receivedMessage.GetMessage()))
}

func TestThatWeListenToStdErrUnixSocket(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	instance := &instance{
		applicationId:       "1234",
		wardenJobId:         56,
		wardenContainerPath: tmpdir,
		index:               4}
	os.MkdirAll(instance.identifier(), 0777)

	stdoutSocketPath := filepath.Join(instance.identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(instance.identifier(), "stderr.sock")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	stdoutListener, err := net.Listen("unix", stdoutSocketPath)
	defer stdoutListener.Close()
	assert.NoError(t, err)
	stderrListener, err := net.Listen("unix", stderrSocketPath)
	defer stderrListener.Close()
	assert.NoError(t, err)

	expectedMessage := "Some Output\n"
	secondLogMessage := "toally different\n"

	mockLoggregatorClient := new(MockLoggregatorClient)

	mockLoggregatorClient.received = make(chan *[]byte)
	instance.startListening(mockLoggregatorClient, logger())

	connection, err := stderrListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(expectedMessage))
	assert.NoError(t, err)

	receivedMessage := getBackendMessage(t, <-mockLoggregatorClient.received)

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, logMessage.LogMessage_DEA, receivedMessage.GetSourceType())
	assert.Equal(t, logMessage.LogMessage_ERR, receivedMessage.GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))

	_, err = connection.Write([]byte(secondLogMessage))
	assert.NoError(t, err)

	receivedMessage = getBackendMessage(t, <-mockLoggregatorClient.received)

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, logMessage.LogMessage_DEA, receivedMessage.GetSourceType())
	assert.Equal(t, logMessage.LogMessage_ERR, receivedMessage.GetMessageType())
	assert.Equal(t, secondLogMessage, string(receivedMessage.GetMessage()))
}
