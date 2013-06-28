package dea_logging_agent

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
	instancesChan := WatchInstancesJsonFileForChanges()

	if _, ok := <-instancesChan; ok {
		t.Error("Found an instance, but should have died")
	}
}

func TestThatAnExistinginstanceWillBeSeen(t *testing.T) {
	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instancesChan := WatchInstancesJsonFileForChanges()

	instance := <-instancesChan
	expectedInstance := &Instance{ApplicationId: "123"}
	assert.Equal(t, expectedInstance, instance)
}

func TestThatANewinstanceWillBeSeen(t *testing.T) {
	file := createFile(t, config.InstancesJsonFilePath)
	defer file.Close()

	instancesChan := WatchInstancesJsonFileForChanges()

	time.Sleep(1 * time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instance, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInstance :=&Instance{ApplicationId: "123"}
	assert.Equal(t, expectedInstance, instance)
}

func TestThatOnlyOneNewInstancesWillBeSeen(t *testing.T) {
	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instancesChan := WatchInstancesJsonFileForChanges()

	instance, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInstance := &Instance{ApplicationId: "123"}
	assert.Equal(t, expectedInstance, instance)

	select {
	case instance = <-instancesChan:
		assert.Nil(t, instance, "We just got an old instance")
	default:
		// OK
	}
}

func TestThatARemovedInstanceWillBeRemoved(t *testing.T) {
	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instancesChan := WatchInstancesJsonFileForChanges()

	instance, ok := <-instancesChan
	assert.True(t, ok, "Channel is closed")
	assert.NotNil(t, instance)


	writeToFile(t, config.InstancesJsonFilePath, `{"instances": []}`, true)

	time.Sleep(2 * time.Millisecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instance, ok = <-instancesChan
	assert.True(t, ok, "Channel is closed")

	expectedInstance := &Instance{ApplicationId: "123"}
	assert.Equal(t, expectedInstance, instance)
}
