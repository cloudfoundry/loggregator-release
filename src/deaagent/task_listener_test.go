package deaagent_test

import (
	"deaagent"
	"deaagent/domain"
	"github.com/cloudfoundry/loggregatorlib/emitter"
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

var testLogger = loggertesthelper.Logger()

func TestThatWeListenToStdOutUnixSocket(t *testing.T) {
	task, tmpdir := setupTask(t, 3)
	defer os.RemoveAll(tmpdir)

	stdoutListener, stderrListener := setupSockets(t, task)
	defer stdoutListener.Close()
	defer stderrListener.Close()

	emitter, receiveChannel := setupEmitter()
	taskListner := deaagent.NewTaskListener(*task, emitter, testLogger)
	go taskListner.StartListening()

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

	emitter, receiveChannel := setupEmitter()
	taskListner := deaagent.NewTaskListener(*task, emitter, testLogger)
	go taskListner.StartListening()

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

	emitter, receiveChannel := setupEmitter()
	taskListner := deaagent.NewTaskListener(*task, emitter, testLogger)
	go taskListner.StartListening()

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

func setupEmitter() (emitter.Emitter, chan *logmessage.LogMessage) {
	mockLoggregatorEmitter := new(MockLoggregatorEmitter)
	mockLoggregatorEmitter.received = make(chan *logmessage.LogMessage)
	return mockLoggregatorEmitter, mockLoggregatorEmitter.received
}

func setupSockets(t *testing.T, task *domain.Task) (net.Listener, net.Listener) {
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

func setupTask(t *testing.T, index uint64) (appTask *domain.Task, tmpdir string) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)

	appTask = &domain.Task{
		ApplicationId:       "1234",
		WardenJobId:         56,
		WardenContainerPath: tmpdir,
		Index:               index,
		SourceName:          "App",
		DrainUrls:           []string{"syslog://10.20.30.40:8050"}}

	os.MkdirAll(appTask.Identifier(), 0777)

	return appTask, tmpdir
}
