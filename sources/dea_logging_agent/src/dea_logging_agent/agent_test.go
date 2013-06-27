package dea_logging_agent

import (
	steno "github.com/cloudfoundry/gosteno"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func init() {
	config = &Config{
		InstancesJsonFilePath: "/tmp/config.json",
		LoggregatorAddress: "localhost:9876"}
	os.Remove(config.InstancesJsonFilePath)
	logger = steno.NewLogger("foobar")
}

func createFile(t *testing.T, name string) *os.File {
	file, error := os.Create(name)
	assert.NoError(t, error)
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
	instanceEventsChan := WatchInstancesJsonFileForChanges()

	if _, ok := <-instanceEventsChan; ok {
		t.Error("Found an instanceEvent, but should have died")
	}
}

func TestThatAnExistinginstanceWillBeSeen(t *testing.T) {
	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instanceEventsChan := WatchInstancesJsonFileForChanges()

	instanceEvent := <-instanceEventsChan
	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(t, expectedInstanceEvent, instanceEvent)
}

func TestThatANewinstanceWillBeSeen(t *testing.T) {
	file := createFile(t, config.InstancesJsonFilePath)
	defer file.Close()

	instanceEventsChan := WatchInstancesJsonFileForChanges()

	time.Sleep(1 * time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instanceEvent, ok := <-instanceEventsChan
	assert.True(t, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, true}
	assert.Equal(t, expectedInstanceEvent, instanceEvent)
}

func TestThatOnlyOneNewInstanceEventWillBeSeen(t *testing.T) {
	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instanceEventsChan := WatchInstancesJsonFileForChanges()

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
	writeToFile(t, config.InstancesJsonFilePath, `{"instances": [{"application_id": "123"}]}`, true)

	instanceEventsChan := WatchInstancesJsonFileForChanges()

	instanceEvent, ok := <-instanceEventsChan
	assert.True(t, ok, "Channel is closed")
	assert.NotNil(t, instanceEvent)

	time.Sleep(2 * time.Millisecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(t, config.InstancesJsonFilePath, `{"instances": []}`, true)

	instanceEvent, ok = <-instanceEventsChan
	assert.True(t, ok, "Channel is closed")

	expectedInstanceEvent := InstanceEvent{Instance{ApplicationId: "123"}, false}
	assert.Equal(t, expectedInstanceEvent, instanceEvent)
}
