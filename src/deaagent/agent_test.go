package deaagent

import (
	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/logmessage"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

var tmpdir string

func init() {
	var err error
	tmpdir, err = ioutil.TempDir("", "testing")
	if err != nil {
		panic(err)
	}
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
	return tmpdir + "/config.json"
}

func loggregatorAddress() string {
	return "localhost:9876"
}

func TestNewAgent(t *testing.T) {
	actualAgent := NewAgent("path", loggertesthelper.Logger())
	expectedAgent := &agent{"path", loggertesthelper.Logger()}
	assert.Equal(t, expectedAgent, actualAgent)
}

func TestTheAgentMonitorsChangesInTasks(t *testing.T) {
	helperTask1 := &task{
		applicationId:       "1234",
		sourceName:          "App",
		wardenJobId:         56,
		wardenContainerPath: tmpdir,
		index:               3}
	os.MkdirAll(helperTask1.identifier(), 0777)

	task1StdoutSocketPath := filepath.Join(helperTask1.identifier(), "stdout.sock")
	task1StderrSocketPath := filepath.Join(helperTask1.identifier(), "stderr.sock")
	os.Remove(task1StdoutSocketPath)
	os.Remove(task1StderrSocketPath)
	task1StdoutListener, err := net.Listen("unix", task1StdoutSocketPath)
	defer task1StdoutListener.Close()
	assert.NoError(t, err)
	task1StderrListener, err := net.Listen("unix", task1StderrSocketPath)
	defer task1StderrListener.Close()
	assert.NoError(t, err)

	helperTask2 := &task{
		applicationId:       "5678",
		sourceName:          "App",
		wardenJobId:         58,
		wardenContainerPath: tmpdir,
		index:               0}
	os.MkdirAll(helperTask2.identifier(), 0777)

	task2StdoutSocketPath := filepath.Join(helperTask2.identifier(), "stdout.sock")
	task2StderrSocketPath := filepath.Join(helperTask2.identifier(), "stderr.sock")
	os.Remove(task2StdoutSocketPath)
	os.Remove(task2StderrSocketPath)
	task2StdoutListener, err := net.Listen("unix", task2StdoutSocketPath)
	defer task2StdoutListener.Close()
	assert.NoError(t, err)
	task2StderrListener, err := net.Listen("unix", task2StderrSocketPath)
	defer task2StderrListener.Close()
	assert.NoError(t, err)

	expectedMessage := "Some Output\n"

	mockLoggregatorEmitter := new(MockLoggregatorEmitter)

	mockLoggregatorEmitter.received = make(chan *logmessage.LogMessage, 2)

	writeToFile(t, `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3}]}`, true)

	agent := NewAgent(filePath(), loggertesthelper.Logger())
	go agent.Start(mockLoggregatorEmitter)

	task1Connection, err := task1StdoutListener.Accept()
	defer task1Connection.Close()
	assert.NoError(t, err)

	writeToFile(t, `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3},
	                               {"state": "RUNNING", "application_id": "5678", "warden_job_id": 58, "warden_container_path":"`+tmpdir+`", "instance_index": 0}]}`, true)

	task2Connection, err := task2StdoutListener.Accept()
	defer task2Connection.Close()
	assert.NoError(t, err)

	_, err = task1Connection.Write([]byte(expectedMessage))
	assert.NoError(t, err)

	_, err = task2Connection.Write([]byte(expectedMessage))
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

func TestTheAgentReadsAllExistingTasks(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "samples", "multi_instances.json")
	testAgent := &agent{InstancesJsonFilePath: filepath, logger: loggertesthelper.Logger()}
	tasksChan := testAgent.watchInstancesJsonFileForChanges()
	expectedApplicationIds := [3]string{
		"e0e12b41-78d4-43ff-a5ae-20422bedf22f",
		"d8df836e-e27d-45d4-a890-b2ce899788a4",
		"a59ebe7a-002a-4530-8d69-8bf53bc845d5",
	}
	for _, expectedTask := range expectedApplicationIds {
		task := <-tasksChan
		assert.Equal(t, expectedTask, task.applicationId)
	}
}

func TestThatFunctionContinuesToPollWhenFileCantBeOpened(t *testing.T) {
	os.Remove(filePath())
	agent := &agent{filePath(), loggertesthelper.Logger()}

	tasksChan := agent.watchInstancesJsonFileForChanges()

	select {
	case <-tasksChan:
		t.Error("Should not have any tasks, the file doesn't exist")
	case <-time.After(2 * time.Second):
		// OK
	}

	writeToFile(t, `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path": "/tmp", "instance_index": 3}]}`, false)

	select {
	case task := <-tasksChan:
		assert.NotNil(t, task)
	case <-time.After(2 * time.Second):
		t.Error("Should have gotten an task by now.")
	}
}

func TestThatAnExistingtaskWillBeSeen(t *testing.T) {
	writeToFile(t, `{"instances": [{"state": "RUNNING", "instance_index": 123}]}`, true)
	agent := &agent{filePath(), loggertesthelper.Logger()}

	tasksChan := agent.watchInstancesJsonFileForChanges()

	inst := <-tasksChan
	expectedInst := task{index: 123, sourceName: "App"}
	assert.Equal(t, expectedInst, inst)
}

func TestThatANewtaskWillBeSeen(t *testing.T) {
	file := createFile(t)
	defer file.Close()
	agent := &agent{filePath(), loggertesthelper.Logger()}

	tasksChan := agent.watchInstancesJsonFileForChanges()

	time.Sleep(1 * time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, `{"instances": [{"state": "RUNNING", "instance_index": 123}]}`, true)

	inst, ok := <-tasksChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := task{index: 123, sourceName: "App"}
	assert.Equal(t, expectedInst, inst)
}

func TestThatOnlyOneNewTasksWillBeSeen(t *testing.T) {
	writeToFile(t, `{"instances": [{"state": "RUNNING", "instance_index": 123}]}`, true)
	agent := &agent{filePath(), loggertesthelper.Logger()}

	tasksChan := agent.watchInstancesJsonFileForChanges()

	inst, ok := <-tasksChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := task{index: 123, sourceName: "App"}
	assert.Equal(t, expectedInst, inst)

	select {
	case inst = <-tasksChan:
		assert.Nil(t, inst, "We just got an old task")
	default:
		// OK
	}
}

func TestThatARemovedTaskWillBeRemoved(t *testing.T) {
	writeToFile(t, `{"instances": [{"state": "RUNNING", "instance_index": 123}]}`, true)
	agent := &agent{filePath(), loggertesthelper.Logger()}

	tasksChan := agent.watchInstancesJsonFileForChanges()

	inst, ok := <-tasksChan
	assert.True(t, ok, "Channel is closed")
	assert.NotNil(t, inst)

	os.Remove(filePath())

	select {
	case inst = <-tasksChan:
		t.Errorf("We just got an old task: %v", inst)
	case <-time.After(2 * time.Millisecond):
		// OK
	}

	writeToFile(t, `{"instances": [{"state": "RUNNING", "instance_index": 1234}]}`, true)

	inst, ok = <-tasksChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := task{index: 1234, sourceName: "App"}
	assert.Equal(t, expectedInst, inst)
}
