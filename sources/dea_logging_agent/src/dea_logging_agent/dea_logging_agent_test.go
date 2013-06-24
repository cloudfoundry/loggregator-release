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

func createFile(t *testing.T, name string) (*os.File) {
	file, error := os.Create(name)
	assert.NoError(t, error)
	return file
}

func writeToFile(t *testing.T, filePath string, text string, truncate bool) {
	var err error

	file := createFile(t, filePath)
	defer file.Close()

	if (truncate) {
		file.Truncate(0)
	}

	_, err = file.WriteString(text)
	assert.NoError(t, err)
}

func TestThatFunctionExistsWhenFileCantBeOpened(t *testing.T) {
	config := initializeConfig("/tmp/config.json")

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	if _, ok := <-instanceEventsChan; ok {
		t.Error("Found an instanceEvent, but should have died")
	}
}

func TestThatAnExistinginstanceWillBeSeen(t *testing.T) {
	config := initializeConfig("/tmp/config.json")
	writeToFile(t, config.instancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	instanceEvent := <-instanceEventsChan;
	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(t, expectedInstanceEvent, instanceEvent)
}

func TestThatANewinstanceWillBeSeen(t *testing.T) {
	config := initializeConfig("/tmp/config.json")
	file := createFile(t, config.instancesJsonFilePath)
	defer file.Close()

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	time.Sleep(1*time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, config.instancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instanceEvent, ok := <-instanceEventsChan
	assert.True(t, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(t, expectedInstanceEvent, instanceEvent)
}

func TestThatOnlyOneNewInstanceEventWillBeSeen(t *testing.T) {
	config := initializeConfig("/tmp/config.json")
	writeToFile(t, config.instancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	instanceEvent, ok := <-instanceEventsChan
	assert.True(t, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(t, expectedInstanceEvent, instanceEvent)

	select {
	case instanceEvent = <-instanceEventsChan:
		assert.Nil(t, instanceEvent, "We just got an old instanceEvent")
	default:
		// OK
	}
}

func TestThatARemovedInstanceWillBeRemoved(t *testing.T) {
	config := initializeConfig("/tmp/config.json")
	writeToFile(t, config.instancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	instanceEvent, ok := <-instanceEventsChan
	assert.True(t, ok, "Channel is closed")
	assert.NotNil(t, instanceEvent)

	time.Sleep(2*time.Millisecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, config.instancesJsonFilePath, `{"instances": []}`, true)

	instanceEvent, ok = <-instanceEventsChan
	assert.True(t, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, false}
	assert.Equal(t, expectedInstanceEvent, instanceEvent)
}
