package dea_logging_agent

import (
	"testing"
    "github.com/stretchr/testify/assert"
	"os"
	"time"
)

func initializeConfig(filePath string) Config {
	config := Config{instancesJsonFilePath: filePath}
	os.Remove(config.instancesJsonFilePath)
	return config
}

func createFile(name string, testState *testing.T) (*os.File) {
	file, error := os.Create(name)
	assert.NoError(testState, error)
	return file
}

func writeToFile(filePath string, text string, testState *testing.T, truncate bool) {
	var err error

	file := createFile(filePath, testState)
	defer file.Close()

	if (truncate) {
		file.Truncate(0)
	}

	_, err = file.WriteString(text)
	assert.NoError(testState, err)
}

func TestThatFunctionExistsWhenFileCantBeOpened(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	if _, ok := <-instanceEventsChan; ok {
		testState.Error("Found an instanceEvent, but should have died")
	}
}

func TestThatAnExistinginstanceWillBeSeen(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")
	writeToFile(config.instancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, testState, true)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	instanceEvent := <-instanceEventsChan;
	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(testState, expectedInstanceEvent, instanceEvent)
}

func TestThatANewinstanceWillBeSeen(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")
	file := createFile(config.instancesJsonFilePath, testState)
	defer file.Close()

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	time.Sleep(1*time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(config.instancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, testState, true)

	instanceEvent, ok := <-instanceEventsChan
	assert.True(testState, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(testState, expectedInstanceEvent, instanceEvent)
}

func TestThatOnlyOneNewInstanceEventWillBeSeen(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")
	writeToFile(config.instancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, testState, true)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	instanceEvent, ok := <-instanceEventsChan
	assert.True(testState, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(testState, expectedInstanceEvent, instanceEvent)

	select {
	case instanceEvent = <-instanceEventsChan:
		assert.Nil(testState, instanceEvent, "We just got an old instanceEvent")
	default:
		// OK
	}
}

func TestThatARemovedInstanceWillBeRemoved(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")
	writeToFile(config.instancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, testState, true)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	instanceEvent, ok := <-instanceEventsChan
	assert.True(testState, ok, "Channel is closed")
	assert.NotNil(testState, instanceEvent)

	time.Sleep(2*time.Millisecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(config.instancesJsonFilePath, `{"instances": []}`, testState, true)

	instanceEvent, ok = <-instanceEventsChan
	assert.True(testState, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, false}
	assert.Equal(testState, expectedInstanceEvent, instanceEvent)
}
