package deaagent

import (
	testhelpers "deaagent_testhelpers"
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
	actualAgent := NewAgent("path", testhelpers.Logger())
	expectedAgent := &agent{"path", testhelpers.Logger()}
	assert.Equal(t, expectedAgent, actualAgent)
}

func TestTheAgentMonitorsChangesInInstances(t *testing.T) {
	//	defer os.RemoveAll(tmpdir)

	helperInstance := &instance{
		applicationId:       "1234",
		wardenJobId:         56,
		wardenContainerPath: tmpdir,
		index:               3}
	os.MkdirAll(helperInstance.identifier(), 0777)

	stdoutSocketPath := filepath.Join(helperInstance.identifier(), "stdout.sock")
	stderrSocketPath := filepath.Join(helperInstance.identifier(), "stderr.sock")
	os.Remove(stdoutSocketPath)
	os.Remove(stderrSocketPath)
	stdoutListener, err := net.Listen("unix", stdoutSocketPath)
	defer stdoutListener.Close()
	assert.NoError(t, err)
	stderrListener, err := net.Listen("unix", stderrSocketPath)
	defer stderrListener.Close()
	assert.NoError(t, err)

	expectedMessage := "Some Output\n"

	mockLoggregatorClient := new(MockLoggregatorClient)

	mockLoggregatorClient.received = make(chan *[]byte)

	writeToFile(t, `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3}]}`, true)
	agent := NewAgent(filePath(), testhelpers.Logger())
	go agent.Start(mockLoggregatorClient)

	connection, err := stdoutListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(expectedMessage))
	assert.NoError(t, err)

	receivedMessage := testhelpers.GetBackendMessage(t, <-mockLoggregatorClient.received)

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, logmessage.LogMessage_WARDEN_CONTAINER, receivedMessage.GetSourceType())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessage.GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
}

func TestTheAgentReadsAllExistingInstances(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "samples", "multi_instances.json")
	testAgent := &agent{InstancesJsonFilePath: filepath, logger: testhelpers.Logger()}
	instancesChan := testAgent.watchInstancesJsonFileForChanges()
	expectedApplicationIds := [3]string{
		"e0e12b41-78d4-43ff-a5ae-20422bedf22f",
		"d8df836e-e27d-45d4-a890-b2ce899788a4",
		"a59ebe7a-002a-4530-8d69-8bf53bc845d5",
	}
	for _, expectedInstance := range expectedApplicationIds {
		instance := <-instancesChan
		assert.Equal(t, expectedInstance, instance.applicationId)
	}
}

func TestThatFunctionContinuesToPollWhenFileCantBeOpened(t *testing.T) {
	os.Remove(filePath())
	agent := &agent{filePath(), testhelpers.Logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	select {
	case <-instancesChan:
		t.Error("Should not have any instances, the file doesn't exist")
	case <-time.After(2 * time.Second):
		// OK
	}

	writeToFile(t, `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path": "/tmp", "instance_index": 3}]}`, false)

	select {
	case instance := <-instancesChan:
		assert.NotNil(t, instance)
	case <-time.After(2 * time.Second):
		t.Error("Should have gotten an instance by now.")
	}
}

func TestThatAnExistinginstanceWillBeSeen(t *testing.T) {
	writeToFile(t, `{"instances": [{"state": "RUNNING", "instance_index": 123}]}`, true)
	agent := &agent{filePath(), testhelpers.Logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	inst := <-instancesChan
	expectedInst := instance{index: 123}
	assert.Equal(t, expectedInst, inst)
}

func TestThatANewinstanceWillBeSeen(t *testing.T) {
	file := createFile(t)
	defer file.Close()
	agent := &agent{filePath(), testhelpers.Logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	time.Sleep(1 * time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, `{"instances": [{"state": "RUNNING", "instance_index": 123}]}`, true)

	inst, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := instance{index: 123}
	assert.Equal(t, expectedInst, inst)
}

func TestThatOnlyOneNewInstancesWillBeSeen(t *testing.T) {
	writeToFile(t, `{"instances": [{"state": "RUNNING", "instance_index": 123}]}`, true)
	agent := &agent{filePath(), testhelpers.Logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	inst, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := instance{index: 123}
	assert.Equal(t, expectedInst, inst)

	select {
	case inst = <-instancesChan:
		assert.Nil(t, inst, "We just got an old instance")
	default:
		// OK
	}
}

func TestThatARemovedInstanceWillBeRemoved(t *testing.T) {
	writeToFile(t, `{"instances": [{"state": "RUNNING", "instance_index": 123}]}`, true)
	agent := &agent{filePath(), testhelpers.Logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	inst, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")
	assert.NotNil(t, inst)

	os.Remove(filePath())

	select {
	case inst = <-instancesChan:
		t.Errorf("We just got an old instance: %v", inst)
	case <-time.After(2 * time.Millisecond):
		// OK
	}

	writeToFile(t, `{"instances": [{"state": "RUNNING", "instance_index": 1234}]}`, true)

	inst, ok = <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := instance{index: 1234}
	assert.Equal(t, expectedInst, inst)
}
