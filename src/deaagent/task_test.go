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
	task := Task{
		ApplicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		WardenJobId:         272,
		WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}

	assert.Equal(t, "/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272", task.Identifier())
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

	expectedMessage := "Some Output"
	secondLogMessage := "toally different"

	connection, err := stdoutListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(SOCKET_PREFIX + expectedMessage))
	_, err = connection.Write([]byte("\n"))
	assert.NoError(t, err)

	receivedMessage := <-receiveChannel

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, "App", receivedMessage.GetSourceName())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessage.GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
	assert.Equal(t, []string{"syslog://10.20.30.40:8050"}, receivedMessage.GetDrainUrls())
	assert.Equal(t, "3", receivedMessage.GetSourceId())

	_, err = connection.Write([]byte(secondLogMessage))
	_, err = connection.Write([]byte("\n"))
	assert.NoError(t, err)

	receivedMessage = <-receiveChannel

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, "App", receivedMessage.GetSourceName())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessage.GetMessageType())
	assert.Equal(t, secondLogMessage, string(receivedMessage.GetMessage()))
	assert.Equal(t, []string{"syslog://10.20.30.40:8050"}, receivedMessage.GetDrainUrls())
	assert.Equal(t, "3", receivedMessage.GetSourceId())

	_, err = connection.Write([]byte("a single line\nthat should be split\n"))
	assert.NoError(t, err)

	receivedMessage = <-receiveChannel
	assert.Equal(t, "a single line", string(receivedMessage.GetMessage()))
	receivedMessage = <-receiveChannel
	assert.Equal(t, "that should be split", string(receivedMessage.GetMessage()))
}

func TestThatWeHandleFourByteOffset(t *testing.T) {
	task, tmpdir := setupTask(t, 3)
	defer os.RemoveAll(tmpdir)

	stdoutListener, stderrListener := setupSockets(t, task)
	defer stdoutListener.Close()
	defer stderrListener.Close()

	receiveChannel := setupEmitter(t, task, testLogger)

	expectedMessage := "Some Output"
	secondLogMessage := "toally different"

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
	_, err = connection.Write([]byte("\n"))
	assert.NoError(t, err)

	receivedMessage := <-receiveChannel
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))

	_, err = connection.Write([]byte(secondLogMessage))
	_, err = connection.Write([]byte("\n"))
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

	expectedMessage := "Some Output"
	secondLogMessage := "toally different"

	receiveChannel := setupEmitter(t, task, testLogger)

	connection, err := stderrListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(SOCKET_PREFIX + expectedMessage))
	_, err = connection.Write([]byte("\n"))
	assert.NoError(t, err)
	receivedMessage := <-receiveChannel
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))

	_, err = connection.Write([]byte(secondLogMessage))
	_, err = connection.Write([]byte("\n"))
	assert.NoError(t, err)
	receivedMessage = <-receiveChannel
	assert.Equal(t, secondLogMessage, string(receivedMessage.GetMessage()))
}

func setupEmitter(t *testing.T, task *Task, logger *gosteno.Logger) chan *logmessage.LogMessage {
	mockLoggregatorEmitter := new(MockLoggregatorEmitter)

	mockLoggregatorEmitter.received = make(chan *logmessage.LogMessage)
	go task.startListening(mockLoggregatorEmitter, logger)
	return mockLoggregatorEmitter.received
}

func setupSockets(t *testing.T, task *Task) (net.Listener, net.Listener) {
	stdoutSocketPath := filepath.Join(task.Identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(task.Identifier(), "stderr.sock")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	stdoutListener, err := net.Listen("unix", stdoutSocketPath)
	assert.NoError(t, err)
	stderrListener, err := net.Listen("unix", stderrSocketPath)
	assert.NoError(t, err)
	return stdoutListener, stderrListener
}

func setupTask(t *testing.T, index uint64) (appTask *Task, tmpdir string) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)

	appTask = &Task{
		ApplicationId:       "1234",
		WardenJobId:         56,
		WardenContainerPath: tmpdir,
		Index:               index,
		SourceName:          "App",
		DrainUrls:           []string{"syslog://10.20.30.40:8050"}}

	os.MkdirAll(appTask.Identifier(), 0777)

	return appTask, tmpdir
}
