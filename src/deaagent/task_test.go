package deaagent

import (
	"github.com/cloudfoundry/gosteno"
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

const SOCKET_PREFIX = "\n\n\n\n"

var testLogger = loggertesthelper.Logger()

func TestIdentifier(t *testing.T) {
	task := task{
		applicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		wardenJobId:         272,
		wardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}

	assert.Equal(t, "/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272", task.identifier())
}

type MockLoggregatorEmitter struct {
	received chan *logmessage.LogMessage
}

func (m MockLoggregatorEmitter) Emit(a, b string) {

}

func (m MockLoggregatorEmitter) EmitError(a, b string) {

}

func (m MockLoggregatorEmitter) EmitLogMessage(message *logmessage.LogMessage) {
	m.received <- message
}

func TestThatWeListenToStdOutUnixSocket(t *testing.T) {
	task, tmpdir := setupTask(t, 3)
	defer os.RemoveAll(tmpdir)

	stdoutListener, stderrListener := setupSockets(t, task)
	defer stdoutListener.Close()
	defer stderrListener.Close()

	receiveChannel := setupEmitter(t, task, testLogger)

	expectedMessage := "Some Output\n"
	secondLogMessage := "toally different\n"

	connection, err := stdoutListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(SOCKET_PREFIX + expectedMessage))
	assert.NoError(t, err)

	receivedMessage := <-receiveChannel

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, "App", receivedMessage.GetSourceName())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessage.GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
	assert.Equal(t, []string{"syslog://10.20.30.40:8050"}, receivedMessage.GetDrainUrls())
	assert.Equal(t, "3", receivedMessage.GetSourceId())

	_, err = connection.Write([]byte(secondLogMessage))
	assert.NoError(t, err)

	receivedMessage = <-receiveChannel

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, "App", receivedMessage.GetSourceName())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessage.GetMessageType())
	assert.Equal(t, secondLogMessage, string(receivedMessage.GetMessage()))
	assert.Equal(t, []string{"syslog://10.20.30.40:8050"}, receivedMessage.GetDrainUrls())
	assert.Equal(t, "3", receivedMessage.GetSourceId())
}

func TestThatWeHandleFourByteOffset(t *testing.T) {
	task, tmpdir := setupTask(t, 3)
	defer os.RemoveAll(tmpdir)

	stdoutListener, stderrListener := setupSockets(t, task)
	defer stdoutListener.Close()
	defer stderrListener.Close()

	receiveChannel := setupEmitter(t, task, testLogger)

	expectedMessage := "Some Output\n"
	secondLogMessage := "toally different\n"

	connection, err := stdoutListener.Accept()
	defer connection.Close()

	assert.NoError(t, err)

	_, err = connection.Write([]byte("\n"))
	_, err = connection.Write([]byte("\n"))

	select {
	case _ = <-receiveChannel:
		t.Error("Should not receive a message")
	case <-time.After(200 * time.Millisecond):
	}

	_, err = connection.Write([]byte("\n\n"))
	_, err = connection.Write([]byte(expectedMessage))
	assert.NoError(t, err)

	receivedMessage := <-receiveChannel
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))

	_, err = connection.Write([]byte(secondLogMessage))
	assert.NoError(t, err)

	receivedMessage = <-receiveChannel
	assert.Equal(t, secondLogMessage, string(receivedMessage.GetMessage()))
}

func TestThatWeListenToStdErrUnixSocket(t *testing.T) {
	task, tmpdir := setupTask(t, 4)
	defer os.RemoveAll(tmpdir)

	stdoutListener, stderrListener := setupSockets(t, task)
	defer stdoutListener.Close()
	defer stderrListener.Close()

	expectedMessage := "Some Output\n"
	secondLogMessage := "toally different\n"

	receiveChannel := setupEmitter(t, task, testLogger)

	connection, err := stderrListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(SOCKET_PREFIX + expectedMessage))
	assert.NoError(t, err)
	receivedMessage := <-receiveChannel
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))

	_, err = connection.Write([]byte(secondLogMessage))
	assert.NoError(t, err)
	receivedMessage = <-receiveChannel
	assert.Equal(t, secondLogMessage, string(receivedMessage.GetMessage()))
}

func TestThatWeRetryListeningToStdOutUnixSocket(t *testing.T) {
	task, tmpdir := setupTask(t, 3)
	defer os.RemoveAll(tmpdir)

	stdoutSocketPath := filepath.Join(task.identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(task.identifier(), "stderr.sock")
	loggerPath := filepath.Join(tmpdir, "logger")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	os.Remove(loggerPath)

	receiveChannel := setupEmitter(t, task, testLogger)

	go func() {
		time.Sleep(950 * time.Millisecond)
		stdoutListener, err := net.Listen("unix", stdoutSocketPath)
		defer stdoutListener.Close()
		assert.NoError(t, err)
		connection, err := stdoutListener.Accept()
		defer connection.Close()
		assert.NoError(t, err)
		_, err = connection.Write([]byte(SOCKET_PREFIX + "Hi there"))
	}()

	select {
	case receivedMessage := <-receiveChannel:
		assert.Contains(t, string(receivedMessage.GetMessage()), "Hi there")
	case <-time.After(3 * time.Second):
		t.Error("Timed out waiting for message")
	}

	logContents := loggertesthelper.TestLoggerSink.LogContents()
	assert.Contains(t, string(logContents), "Error while dialing into socket OUT")
}

func TestThatWeRetryListeningToStdErrUnixSocket(t *testing.T) {
	task, tmpdir := setupTask(t, 3)
	defer os.RemoveAll(tmpdir)

	stdoutSocketPath := filepath.Join(task.identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(task.identifier(), "stderr.sock")
	loggerPath := filepath.Join(tmpdir, "logger")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	os.Remove(loggerPath)

	receiveChannel := setupEmitter(t, task, testLogger)

	go func() {
		time.Sleep(950 * time.Millisecond)
		stderrListener, err := net.Listen("unix", stderrSocketPath)
		defer stderrListener.Close()
		assert.NoError(t, err)
		connection, err := stderrListener.Accept()
		defer connection.Close()
		assert.NoError(t, err)
		_, err = connection.Write([]byte(SOCKET_PREFIX + "Error Message!!!"))
	}()

	select {
	case receivedMessage := <-receiveChannel:
		assert.Contains(t, string(receivedMessage.GetMessage()), "Error Message!!!")
	case <-time.After(3 * time.Second):
		t.Error("Timed out waiting for message")
	}

	logContents := loggertesthelper.TestLoggerSink.LogContents()
	assert.Contains(t, string(logContents), "Error while dialing into socket ERR")
}

func setupEmitter(t *testing.T, task *task, logger *gosteno.Logger) chan *logmessage.LogMessage {
	mockLoggregatorEmitter := new(MockLoggregatorEmitter)

	mockLoggregatorEmitter.received = make(chan *logmessage.LogMessage)
	task.startListening(mockLoggregatorEmitter, logger)
	return mockLoggregatorEmitter.received
}

func setupSockets(t *testing.T, task *task) (net.Listener, net.Listener) {
	stdoutSocketPath := filepath.Join(task.identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(task.identifier(), "stderr.sock")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	stdoutListener, err := net.Listen("unix", stdoutSocketPath)
	assert.NoError(t, err)
	stderrListener, err := net.Listen("unix", stderrSocketPath)
	assert.NoError(t, err)
	return stdoutListener, stderrListener
}

func setupTask(t *testing.T, index uint64) (appTask *task, tmpdir string) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)

	appTask = &task{
		applicationId:       "1234",
		wardenJobId:         56,
		wardenContainerPath: tmpdir,
		index:               index,
		sourceName:          "App",
		drainUrls:           []string{"syslog://10.20.30.40:8050"}}

	os.MkdirAll(appTask.identifier(), 0777)

	return appTask, tmpdir
}
