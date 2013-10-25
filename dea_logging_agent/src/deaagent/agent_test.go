package deaagent

import (
	testhelpers "deaagent_testhelpers"
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

func TestTheAgentMonitorsChangesInInstances(t *testing.T) {
	helperInstance1 := &instance{
		applicationId:       "1234",
		wardenJobId:         56,
		wardenContainerPath: tmpdir,
		index:               3}
	os.MkdirAll(helperInstance1.identifier(), 0777)

	instance1StdoutSocketPath := filepath.Join(helperInstance1.identifier(), "stdout.sock")
	instance1StderrSocketPath := filepath.Join(helperInstance1.identifier(), "stderr.sock")
	os.Remove(instance1StdoutSocketPath)
	os.Remove(instance1StderrSocketPath)
	instance1StdoutListener, err := net.Listen("unix", instance1StdoutSocketPath)
	defer instance1StdoutListener.Close()
	assert.NoError(t, err)
	instance1StderrListener, err := net.Listen("unix", instance1StderrSocketPath)
	defer instance1StderrListener.Close()
	assert.NoError(t, err)

	helperInstance2 := &instance{
		applicationId:       "5678",
		wardenJobId:         58,
		wardenContainerPath: tmpdir,
		index:               0}
	os.MkdirAll(helperInstance2.identifier(), 0777)

	instance2StdoutSocketPath := filepath.Join(helperInstance2.identifier(), "stdout.sock")
	instance2StderrSocketPath := filepath.Join(helperInstance2.identifier(), "stderr.sock")
	os.Remove(instance2StdoutSocketPath)
	os.Remove(instance2StderrSocketPath)
	instance2StdoutListener, err := net.Listen("unix", instance2StdoutSocketPath)
	defer instance2StdoutListener.Close()
	assert.NoError(t, err)
	instance2StderrListener, err := net.Listen("unix", instance2StderrSocketPath)
	defer instance2StderrListener.Close()
	assert.NoError(t, err)

	expectedMessage := "Some Output\n"

	mockLoggregatorClient := new(MockLoggregatorClient)

	mockLoggregatorClient.received = make(chan *[]byte, 2)

	writeToFile(t, `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3}]}`, true)

	agent := NewAgent(filePath(), loggertesthelper.Logger())
	go agent.Start(mockLoggregatorClient)

	instance1Connection, err := instance1StdoutListener.Accept()
	defer instance1Connection.Close()
	assert.NoError(t, err)

	writeToFile(t, `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3},
	                               {"state": "RUNNING", "application_id": "5678", "warden_job_id": 58, "warden_container_path":"`+tmpdir+`", "instance_index": 0}]}`, true)

	instance2Connection, err := instance2StdoutListener.Accept()
	defer instance2Connection.Close()
	assert.NoError(t, err)

	_, err = instance1Connection.Write([]byte(expectedMessage))
	assert.NoError(t, err)

	_, err = instance2Connection.Write([]byte(expectedMessage))
	assert.NoError(t, err)

	receivedMessages := make(map[string]*logmessage.LogMessage)

	receivedMessage := testhelpers.GetBackendMessage(t, <-mockLoggregatorClient.received)
	receivedMessages[receivedMessage.GetAppId()] = receivedMessage

	receivedMessage = testhelpers.GetBackendMessage(t, <-mockLoggregatorClient.received)
	receivedMessages[receivedMessage.GetAppId()] = receivedMessage

	assert.Equal(t, 2, len(receivedMessages))

	assert.NotNil(t, receivedMessages["1234"])
	assert.Equal(t, logmessage.LogMessage_WARDEN_CONTAINER, receivedMessages["1234"].GetSourceType())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessages["1234"].GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessages["1234"].GetMessage()))

	assert.NotNil(t, receivedMessages["5678"])
	assert.Equal(t, logmessage.LogMessage_WARDEN_CONTAINER, receivedMessages["5678"].GetSourceType())
	assert.Equal(t, logmessage.LogMessage_OUT, receivedMessages["5678"].GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessages["5678"].GetMessage()))
}

func TestTheAgentReadsAllExistingInstances(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "samples", "multi_instances.json")
	testAgent := &agent{InstancesJsonFilePath: filepath, logger: loggertesthelper.Logger()}
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
	agent := &agent{filePath(), loggertesthelper.Logger()}

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
	agent := &agent{filePath(), loggertesthelper.Logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	inst := <-instancesChan
	expectedInst := instance{index: 123}
	assert.Equal(t, expectedInst, inst)
}

func TestThatANewinstanceWillBeSeen(t *testing.T) {
	file := createFile(t)
	defer file.Close()
	agent := &agent{filePath(), loggertesthelper.Logger()}

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
	agent := &agent{filePath(), loggertesthelper.Logger()}

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
	agent := &agent{filePath(), loggertesthelper.Logger()}

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
