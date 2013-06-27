package dea_logging_agent

import (
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
	received chan *[]byte
}

func (m *MockLoggregatorClient) Send(data []byte) {
	m.received <- &data
}

func TestThatWeListenToStdOutUnixSocket(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	instance := &Instance{
		ApplicationId:       "1234",
		WardenJobId:         "56",
		WardenContainerPath: tmpdir}
	os.MkdirAll(instance.Identifier(), 0777)

	stdoutSocketPath := filepath.Join(instance.Identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(instance.Identifier(), "stderr.sock")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	stdoutListener, err := net.Listen("unix", stdoutSocketPath)
	defer stdoutListener.Close()
	assert.NoError(t, err)
	stderrListener, err := net.Listen("unix", stderrSocketPath)
	defer stderrListener.Close()
	assert.NoError(t, err)


	logMessage := "Some Output\n"
	secondLogMessage := "toally different\n"

	mockLoggregatorClient := new(MockLoggregatorClient)

	mockLoggregatorClient.received = make(chan *[]byte)
	instance.StartListening(mockLoggregatorClient)

	connection, err := stdoutListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(logMessage))
	assert.NoError(t, err)

	data := <- mockLoggregatorClient.received

	assert.Equal(t, "STDOUT " + logMessage, string(*data))

	_, err = connection.Write([]byte(secondLogMessage))
	assert.NoError(t, err)

	data = <- mockLoggregatorClient.received
	assert.Equal(t, "STDOUT " + secondLogMessage, string(*data))
}

func TestThatWeListenToStdErrUnixSocket(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	instance := &Instance{
		ApplicationId:       "1234",
		WardenJobId:         "56",
		WardenContainerPath: tmpdir}
	os.MkdirAll(instance.Identifier(), 0777)

	stdoutSocketPath := filepath.Join(instance.Identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(instance.Identifier(), "stderr.sock")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	stdoutListener, err := net.Listen("unix", stdoutSocketPath)
	defer stdoutListener.Close()
	assert.NoError(t, err)
	stderrListener, err := net.Listen("unix", stderrSocketPath)
	defer stderrListener.Close()
	assert.NoError(t, err)


	logMessage := "Some Output\n"
	secondLogMessage := "toally different\n"

	mockLoggregatorClient := new(MockLoggregatorClient)

	mockLoggregatorClient.received = make(chan *[]byte)
	instance.StartListening(mockLoggregatorClient)

	connection, err := stderrListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(logMessage))
	assert.NoError(t, err)

	data := <- mockLoggregatorClient.received
	assert.Equal(t, "STDERR " + logMessage, string(*data))

	_, err = connection.Write([]byte(secondLogMessage))
	assert.NoError(t, err)

	data = <- mockLoggregatorClient.received
	assert.Equal(t, "STDERR " + secondLogMessage, string(*data))
}
