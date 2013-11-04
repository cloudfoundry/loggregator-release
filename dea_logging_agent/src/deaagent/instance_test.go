package deaagent

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIdentifier(t *testing.T) {
	instance := instance{
		applicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		wardenJobId:         272,
		wardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}

	assert.Equal(t, "/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272", instance.identifier())
}

type MockLoggregatorEmitter struct {
	received chan *logmessage.LogMessage
}

func (m MockLoggregatorEmitter) Emit(a, b string) {

}

func (m MockLoggregatorEmitter) EmitLogMessage(message *logmessage.LogMessage) {
	m.received <- message
}

func TestThatWeListenToStdOutUnixSocket(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	instance := &instance{
		applicationId:       "1234",
		wardenJobId:         56,
		wardenContainerPath: tmpdir,
		index:               3,
		drainUrls:           []string{"syslog://10.20.30.40:8050"}}

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

	mockLoggregatorEmitter := new(MockLoggregatorEmitter)

	mockLoggregatorEmitter.received = make(chan *logmessage.LogMessage)
	instance.startListening(mockLoggregatorEmitter, loggertesthelper.Logger())

	connection, err := stdoutListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(expectedMessage))
	assert.NoError(t, err)

	receivedMessage := <-mockLoggregatorEmitter.received

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, logmessage.LogMessage_WARDEN_CONTAINER, receivedMessage.GetSourceType())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessage.GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
	assert.Equal(t, []string{"syslog://10.20.30.40:8050"}, receivedMessage.GetDrainUrls())
	assert.Equal(t, "3", receivedMessage.GetSourceId())

	_, err = connection.Write([]byte(secondLogMessage))
	assert.NoError(t, err)

	receivedMessage = <-mockLoggregatorEmitter.received

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, logmessage.LogMessage_WARDEN_CONTAINER, receivedMessage.GetSourceType())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessage.GetMessageType())
	assert.Equal(t, secondLogMessage, string(receivedMessage.GetMessage()))
	assert.Equal(t, []string{"syslog://10.20.30.40:8050"}, receivedMessage.GetDrainUrls())
	assert.Equal(t, "3", receivedMessage.GetSourceId())
}

func TestThatWeListenToStdErrUnixSocket(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	instance := &instance{
		applicationId:       "1234",
		wardenJobId:         56,
		wardenContainerPath: tmpdir,
		index:               4,
		drainUrls:           []string{"syslog://10.20.30.40:8050"}}
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

	mockLoggregatorEmitter := new(MockLoggregatorEmitter)

	mockLoggregatorEmitter.received = make(chan *logmessage.LogMessage)
	instance.startListening(mockLoggregatorEmitter, loggertesthelper.Logger())

	connection, err := stderrListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(expectedMessage))
	assert.NoError(t, err)

	receivedMessage := <-mockLoggregatorEmitter.received

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, logmessage.LogMessage_WARDEN_CONTAINER, receivedMessage.GetSourceType())
	assert.Equal(t, logmessage.LogMessage_ERR, receivedMessage.GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
	assert.Equal(t, []string{"syslog://10.20.30.40:8050"}, receivedMessage.GetDrainUrls())
	assert.Equal(t, "4", receivedMessage.GetSourceId())

	_, err = connection.Write([]byte(secondLogMessage))
	assert.NoError(t, err)

	receivedMessage = <-mockLoggregatorEmitter.received

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, logmessage.LogMessage_WARDEN_CONTAINER, receivedMessage.GetSourceType())
	assert.Equal(t, logmessage.LogMessage_ERR, receivedMessage.GetMessageType())
	assert.Equal(t, secondLogMessage, string(receivedMessage.GetMessage()))
	assert.Equal(t, []string{"syslog://10.20.30.40:8050"}, receivedMessage.GetDrainUrls())
	assert.Equal(t, "4", receivedMessage.GetSourceId())
}

func TestThatWeRetryListeningToStdOutUnixSocket(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	instance := &instance{
		applicationId:       "1234",
		wardenJobId:         56,
		wardenContainerPath: tmpdir,
		index:               3,
		drainUrls:           []string{"syslog://10.20.30.40:8050"},
	}

	os.MkdirAll(instance.identifier(), 0777)

	stdoutSocketPath := filepath.Join(instance.identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(instance.identifier(), "stderr.sock")
	loggerPath := filepath.Join(tmpdir, "logger")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	os.Remove(loggerPath)

	mockLoggregatorEmitter := new(MockLoggregatorEmitter)

	mockLoggregatorEmitter.received = make(chan *logmessage.LogMessage)

	instance.startListening(mockLoggregatorEmitter, loggertesthelper.FileLogger(loggerPath))

	go func() {
		time.Sleep(950 * time.Millisecond)
		stdoutListener, err := net.Listen("unix", stdoutSocketPath)
		defer stdoutListener.Close()
		assert.NoError(t, err)
		connection, err := stdoutListener.Accept()
		defer connection.Close()
		assert.NoError(t, err)
		_, err = connection.Write([]byte("Hi there"))
	}()

	select {
	case receivedMessage := <-mockLoggregatorEmitter.received:
		assert.Contains(t, string(receivedMessage.GetMessage()), "Hi there")
	case <-time.After(3 * time.Second):
		t.Error("Timed out waiting for message")
	}

	logContents, err := ioutil.ReadFile(loggerPath)
	assert.NoError(t, err)
	assert.Contains(t, string(logContents), "Error while dialing into socket OUT")
}

func TestThatWeRetryListeningToStdErrUnixSocket(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	instance := &instance{
		applicationId:       "1234",
		wardenJobId:         56,
		wardenContainerPath: tmpdir,
		index:               3,
		drainUrls:           []string{"syslog://10.20.30.40:8050"},
	}

	os.MkdirAll(instance.identifier(), 0777)

	stdoutSocketPath := filepath.Join(instance.identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(instance.identifier(), "stderr.sock")
	loggerPath := filepath.Join(tmpdir, "logger")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	os.Remove(loggerPath)

	mockLoggregatorEmitter := new(MockLoggregatorEmitter)

	mockLoggregatorEmitter.received = make(chan *logmessage.LogMessage)

	instance.startListening(mockLoggregatorEmitter, loggertesthelper.FileLogger(loggerPath))

	go func() {
		time.Sleep(950 * time.Millisecond)
		stderrListener, err := net.Listen("unix", stderrSocketPath)
		defer stderrListener.Close()
		assert.NoError(t, err)
		connection, err := stderrListener.Accept()
		defer connection.Close()
		assert.NoError(t, err)
		_, err = connection.Write([]byte("Error Message!!!"))
	}()

	select {
	case receivedMessage := <-mockLoggregatorEmitter.received:
		assert.Contains(t, string(receivedMessage.GetMessage()), "Error Message!!!")
	case <-time.After(3 * time.Second):
		t.Error("Timed out waiting for message")
	}

	logContents, err := ioutil.ReadFile(loggerPath)
	assert.NoError(t, err)
	assert.Contains(t, string(logContents), "Error while dialing into socket ERR")
}
