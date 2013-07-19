package deaagent

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"logMessage"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func createFile(t *testing.T, name string) *os.File {
	file, err := os.Create(name)
	assert.NoError(t, err)
	return file
}

func writeToFile(t *testing.T, filePath string, text string, truncate bool) {
	var err error

	file := createFile(t, filePath)
	defer file.Close()

	if truncate {
		file.Truncate(0)
	}

	_, err = file.WriteString(text)
	assert.NoError(t, err)
}

func filePath() string {
	return "/tmp/config.json"
}

func loggregatorAddress() string {
	return "localhost:9876"
}

func TestNewAgent(t *testing.T) {
	actualAgent := NewAgent("path", logger())
	expectedAgent := &agent{"path", logger()}
	assert.Equal(t, expectedAgent, actualAgent)
}

func TestTheAgentMonitorsChangesInInstances(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "testing")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

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

	writeToFile(t, filePath(), `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path":"`+tmpdir+`", "instance_index": 3}]}`, true)
	agent := NewAgent(filePath(), logger())
	go agent.Start(mockLoggregatorClient)

	connection, err := stdoutListener.Accept()
	defer connection.Close()
	assert.NoError(t, err)

	_, err = connection.Write([]byte(expectedMessage))
	assert.NoError(t, err)

	receivedMessage := getBackendMessage(t, <-mockLoggregatorClient.received)

	assert.Equal(t, "1234", receivedMessage.GetAppId())
	assert.Equal(t, logMessage.LogMessage_DEA, receivedMessage.GetSourceType())
	assert.Equal(t, logMessage.LogMessage_OUT, receivedMessage.GetMessageType())
	assert.Equal(t, expectedMessage, string(receivedMessage.GetMessage()))
}

func TestTheAgentReadsAllExistingInstances(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "samples", "multi_instances.json")
	testAgent := &agent{InstancesJsonFilePath: filepath, logger: logger()}
	instancesChan := testAgent.watchInstancesJsonFileForChanges()
	expectedApplicationIds := [8]string{
		"e0e12b41-78d4-43ff-a5ae-20422bedf22f",
		"a59ebe7a-002a-4530-8d69-8bf53bc845d5",
		"01780118-4680-4779-9eb7-65c7f60cdc76",
		"67fa7adf-746a-446a-8f1b-d2f291bc8459",
		"e6eca3c8-7fd1-4876-8545-ce5acf67db66",
		"b88d0b09-8275-445f-bd83-1941362db7aa",
		"d2fb0f4b-81a9-47a4-8eb4-292456030fa3",
		"d3532fe5-1ad8-4418-b6fb-4d72be408b72",
	}
	for i := 0; i > 8; i++ {
		instance := <-instancesChan
		assert.Equal(t, expectedApplicationIds[i], instance.applicationId)
	}
}

func TestThatFunctionContinuesToPollWhenFileCantBeOpened(t *testing.T) {
	os.Remove(filePath())
	agent := &agent{filePath(), logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	select {
	case <-instancesChan:
		t.Error("Should not have any instances, the file doesn't exist")
	default:
		// OK
	}

	writeToFile(t, filePath(), `{"instances": [{"state": "RUNNING", "application_id": "1234", "warden_job_id": 56, "warden_container_path": "/tmp", "instance_index": 3}]}`, true)
	runtime.Gosched()

	select {
	case instance := <-instancesChan:
		assert.NotNil(t, instance)
	case <-time.After(2 * time.Second):
		t.Error("Should have gotten an instance by now.")
	}
}

func TestThatAnExistinginstanceWillBeSeen(t *testing.T) {
	writeToFile(t, filePath(), `{"instances": [{"state": "RUNNING", "application_id": "123"}]}`, true)
	agent := &agent{filePath(), logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	inst := <-instancesChan
	expectedInst := instance{applicationId: "123"}
	assert.Equal(t, expectedInst, inst)
}

func TestThatANewinstanceWillBeSeen(t *testing.T) {
	file := createFile(t, filePath())
	defer file.Close()
	agent := &agent{filePath(), logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	time.Sleep(1 * time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, filePath(), `{"instances": [{"state": "RUNNING", "application_id": "123"}]}`, true)

	inst, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := instance{applicationId: "123"}
	assert.Equal(t, expectedInst, inst)
}

func TestThatOnlyOneNewInstancesWillBeSeen(t *testing.T) {
	writeToFile(t, filePath(), `{"instances": [{"state": "RUNNING", "application_id": "123"}]}`, true)
	agent := &agent{filePath(), logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	inst, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := instance{applicationId: "123"}
	assert.Equal(t, expectedInst, inst)

	select {
	case inst = <-instancesChan:
		assert.Nil(t, inst, "We just got an old instance")
	default:
		// OK
	}
}

func TestThatARemovedInstanceWillBeRemoved(t *testing.T) {
	writeToFile(t, filePath(), `{"instances": [{"state": "RUNNING", "application_id": "123"}]}`, true)
	agent := &agent{filePath(), logger()}

	instancesChan := agent.watchInstancesJsonFileForChanges()

	inst, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")
	assert.NotNil(t, inst)

	writeToFile(t, filePath(), `{"instances": []}`, true)

	time.Sleep(2 * time.Millisecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, filePath(), `{"instances": [{"state": "RUNNING", "application_id": "123"}]}`, true)

	inst, ok = <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := instance{applicationId: "123"}
	assert.Equal(t, expectedInst, inst)
}
