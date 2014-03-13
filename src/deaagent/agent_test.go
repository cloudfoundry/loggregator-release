package deaagent_test

import (
	"deaagent"
	"deaagent/domain"
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

var tmpdir string

func init() {
	var err error
	tmpdir, err = ioutil.TempDir("", "testing")
	if err != nil {
		panic(err)
	}
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

func createFile(t *testing.T) *os.File {
	file, err := os.Create(filePath())
	assert.NoError(t, err)
	return file
}

func writeToFile(t *testing.T, text string, truncate bool) {
	var err error

	file := createFile(t)
	defer file.Close()

	if truncate {
		file.Truncate(0)
	}

	_, err = file.WriteString(text)
	assert.NoError(t, err)
}

func filePath() string {
	return tmpdir + "/instances.json"
}

func loggregatorAddress() string {
	return "localhost:9876"
}

func TestTheAgentMonitorsChangesInTasks(t *testing.T) {
	helperTask1 := &domain.Task{
		ApplicationId:       "1234",
		SourceName:          "App",
		WardenJobId:         56,
		WardenContainerPath: tmpdir,
		Index:               3}
	os.MkdirAll(helperTask1.Identifier(), 0777)

	task1StdoutSocketPath := filepath.Join(helperTask1.Identifier(), "stdout.sock")
	task1StderrSocketPath := filepath.Join(helperTask1.Identifier(), "stderr.sock")
	os.Remove(task1StdoutSocketPath)
	os.Remove(task1StderrSocketPath)
	task1StdoutListener, err := net.Listen("unix", task1StdoutSocketPath)
	defer task1StdoutListener.Close()
	assert.NoError(t, err)
	task1StderrListener, err := net.Listen("unix", task1StderrSocketPath)
	defer task1StderrListener.Close()
	assert.NoError(t, err)

	helperTask2 := &domain.Task{
		ApplicationId:       "5678",
		SourceName:          "App",
		WardenJobId:         58,
		WardenContainerPath: tmpdir,
		Index:               0}
	os.MkdirAll(helperTask2.Identifier(), 0777)

	task2StdoutSocketPath := filepath.Join(helperTask2.Identifier(), "stdout.sock")
	task2StderrSocketPath := filepath.Join(helperTask2.Identifier(), "stderr.sock")
	os.Remove(task2StdoutSocketPath)
	os.Remove(task2StderrSocketPath)
	task2StdoutListener, err := net.Listen("unix", task2StdoutSocketPath)
	defer task2StdoutListener.Close()
	assert.NoError(t, err)
	task2StderrListener, err := net.Listen("unix", task2StderrSocketPath)
	defer task2StderrListener.Close()
	assert.NoError(t, err)

	expectedMessage := "Some Output"

	mockLoggregatorEmitter := new(MockLoggregatorEmitter)

	mockLoggregatorEmitter.received = make(chan *logmessage.LogMessage, 2)

	writeToFile(t, `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3}]}`, true)

	agent := deaagent.NewAgent(filePath(), loggertesthelper.Logger())
	go agent.Start(mockLoggregatorEmitter)

	task1Connection, err := task1StdoutListener.Accept()
	defer task1Connection.Close()
	assert.NoError(t, err)

	writeToFile(t, `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3},
	                               {"state": "RUNNING", "application_id": "5678", "warden_job_id": 58, "warden_container_path":"`+tmpdir+`", "instance_index": 0}]}`, true)

	task2Connection, err := task2StdoutListener.Accept()
	defer task2Connection.Close()
	assert.NoError(t, err)

	_, err = task1Connection.Write([]byte(SOCKET_PREFIX + expectedMessage))
	_, err = task1Connection.Write([]byte("\n"))
	assert.NoError(t, err)

	_, err = task2Connection.Write([]byte(SOCKET_PREFIX + expectedMessage))
	_, err = task2Connection.Write([]byte("\n"))
	assert.NoError(t, err)

	receivedMessages := make(map[string]*logmessage.LogMessage)

	receivedMessage := <-mockLoggregatorEmitter.received
	receivedMessages[receivedMessage.GetAppId()] = receivedMessage

	receivedMessage = <-mockLoggregatorEmitter.received
	receivedMessages[receivedMessage.GetAppId()] = receivedMessage

	assert.Equal(t, 2, len(receivedMessages))

	assert.NotNil(t, receivedMessages["1234"])
	assert.Equal(t, "App", receivedMessages["1234"].GetSourceName())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessages["1234"].GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessages["1234"].GetMessage()))

	assert.NotNil(t, receivedMessages["5678"])
	assert.Equal(t, "App", receivedMessages["5678"].GetSourceName())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessages["5678"].GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessages["5678"].GetMessage()))
}

func xTestThatFunctionContinuesToPollWhenFileCantBeOpened(t *testing.T) {

	task1StdoutSocketPath := filepath.Join("tmp", "jobs", "56", "stdout.sock")
	task1StderrSocketPath := filepath.Join("tmp", "jobs", "56", "stderr.sock")

	os.MkdirAll(filepath.Join("tmp", "jobs", "56"), 0777)

	os.Remove(filePath())
	os.Remove(task1StdoutSocketPath)
	os.Remove(task1StderrSocketPath)
	agent := deaagent.NewAgent(filePath(), loggertesthelper.StdOutLogger())
	mockLoggregatorEmitter := new(MockLoggregatorEmitter)

	mockLoggregatorEmitter.received = make(chan *logmessage.LogMessage, 2)

	go agent.Start(mockLoggregatorEmitter)

	select {
	case <-mockLoggregatorEmitter.received:
		t.Error("Should not have any messages, the file doesn't exist")
	case <-time.After(2 * time.Second):
		// OK
	}

	println("done with the empty file")

	task1StdoutListener, err := net.Listen("unix", task1StdoutSocketPath)
	assert.NoError(t, err)
	task1StderrListener, err := net.Listen("unix", task1StderrSocketPath)
	defer task1StderrListener.Close()
	assert.NoError(t, err)

	go func() {
		task1Connection, err := task1StdoutListener.Accept()
		println("got a socket connection")
		if err != nil {
			println(err.Error())
			assert.NoError(t, err)
			return
		}

		println("writing to the socket connection")
		task1Connection.Write([]byte("a log line\n"))
		println("wrote to the socket connection")
	}()

	println("created all the sockets")

	writeToFile(t, `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path": "/tmp", "instance_index": 3}]}`, false)

	println("updated instances.json")

	select {
	case logMessage := <-mockLoggregatorEmitter.received:
		assert.Equal(t, logMessage.GetMessage(), "a log line")
	case <-time.After(2 * time.Second):
		t.Error("Should have gotten a message by now.")
	}
}
