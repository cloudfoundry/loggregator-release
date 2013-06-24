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

func createFile(name string, config Config, testState *testing.T) *os.File {
	file, error := os.Create(name)
	assert.NoError(testState, error)

	return file
}

func writeToFile(file *os.File, text string, testState *testing.T) {
	_, error := file.WriteString(text)
	assert.NoError(testState, error)
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
	file := createFile(config.instancesJsonFilePath, config, testState)
	writeToFile(file, `{"instances": [{"application_id": "123"}]}`, testState)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	instanceEvent := <-instanceEventsChan;
	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(testState, expectedInstanceEvent, instanceEvent)
}

func TestThatANewinstanceWillBeSeen(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")
	file := createFile(config.instancesJsonFilePath, config, testState)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	time.Sleep(1*time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(file, `{"instances": [{"application_id": "123"}]}`, testState)

	instanceEvent, ok := <-instanceEventsChan
	assert.True(testState, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(testState, expectedInstanceEvent, instanceEvent)
}

func TestThatOnlyNewInstancesWillBeSeen(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")
	file := createFile(config.instancesJsonFilePath, config, testState)
	writeToFile(file, `{"instances": [{"application_id": "123"}]}`, testState)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	instanceEvent, ok := <-instanceEventsChan
	assert.True(testState, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(testState, expectedInstanceEvent, instanceEvent)

	time.Sleep(2*time.Millisecond) // ensure that the go function starts before we add proper data to the json config

	os.Truncate(config.instancesJsonFilePath, 0)
	writeToFile(file, `{"instances": [{"application_id": "123"}]}`, testState)

	select {
	case instanceEvent = <-instanceEventsChan:
		assert.Nil(testState, instanceEvent, "We just got an old instanceEvent")
	default:
		// OK
	}
}

func TestThatARemovedInstanceWillBeRemoved(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")
	file := createFile(config.instancesJsonFilePath, config, testState)
	writeToFile(file, `{"instances": [{"application_id": "123"}]}`, testState)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	time.Sleep(2*time.Millisecond) // ensure that the go function starts before we add proper data to the json config

	instanceEvent, ok := <-instanceEventsChan
	assert.True(testState, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(testState, expectedInstanceEvent, instanceEvent)
}
