package deaagent

import (
	"github.com/stretchr/testify/assert"
	"os"
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

func TestThatFunctionExistsWhenFileCantBeOpened(t *testing.T) {
	instancesChan := watchInstancesJsonFileForChanges()

	if _, ok := <-instancesChan; ok {
		t.Error("Found an instance, but should have died")
	}
}

func TestThatAnExistinginstanceWillBeSeen(t *testing.T) {
	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instancesChan := watchInstancesJsonFileForChanges()

	inst := <-instancesChan
	expectedInst := &instance{applicationId: "123"}
	assert.Equal(t, expectedInst, inst)
}

func TestThatANewinstanceWillBeSeen(t *testing.T) {
	file := createFile(t, config.InstancesJsonFilePath)
	defer file.Close()

	instancesChan := watchInstancesJsonFileForChanges()

	time.Sleep(1 * time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	inst, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInst :=&instance{applicationId: "123"}
	assert.Equal(t, expectedInst, inst)
}

func TestThatOnlyOneNewInstancesWillBeSeen(t *testing.T) {
	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instancesChan := watchInstancesJsonFileForChanges()

	inst, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := &instance{applicationId: "123"}
	assert.Equal(t, expectedInst, inst)

	select {
	case inst = <-instancesChan:
		assert.Nil(t, inst, "We just got an old instance")
	default:
		// OK
	}
}

func TestThatARemovedInstanceWillBeRemoved(t *testing.T) {
	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instancesChan := watchInstancesJsonFileForChanges()

	inst, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")
	assert.NotNil(t, inst)

	writeToFile(t, config.InstancesJsonFilePath, `{"instances": []}`, true)

	time.Sleep(2 * time.Millisecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	inst, ok = <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInst := &instance{applicationId: "123"}
	assert.Equal(t, expectedInst, inst)
}
