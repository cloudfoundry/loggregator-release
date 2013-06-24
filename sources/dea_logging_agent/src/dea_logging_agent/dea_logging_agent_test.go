package dea_logging_agent

import (
	"testing"
	"os"
	"time"
	"reflect"
)

func initializeConfig(filePath string) Config {
	config := Config{instancesJsonFilePath: filePath}
	os.Remove(config.instancesJsonFilePath)
	return config
}

func createFile(name string, config Config, testState *testing.T) *os.File {
	file, error := os.Create(name)
	if error != nil {
		testState.Error(error)
	}
	return file
}

func writeToFile(file *os.File, text string, testState *testing.T) {
	_, error := file.WriteString(text)
	if error != nil {
		testState.Error(error)
	}
}

func assertEquals(testState *testing.T, expected interface{}, actual interface{}) {
	if expected != actual {
		testState.Errorf("%v %v should have been %v", reflect.TypeOf(expected).Name(), actual, expected)
	}
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
	expectedInstanceEvent := InstanceEvent{Instance{"123"}, true}
	if instanceEvent != expectedInstanceEvent {
		testState.Errorf("InstanceEvent %v should have been %v", instanceEvent, expectedInstanceEvent)
	}
}

func TestThatANewinstanceWillBeSeen(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")
	file := createFile(config.instancesJsonFilePath, config, testState)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	time.Sleep(1*time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(file, `{"instances": [{"application_id": "123"}]}`, testState)

	instanceEvent, ok := <-instanceEventsChan
	if !ok {
		testState.Error("There was no instanceEvent!")
	}
	expectedInstanceEvent := InstanceEvent{Instance{"123"}, true}
	assertEquals(testState, expectedInstanceEvent, instanceEvent)
}

func TestThatOnlyNewInstancesWillBeSeen(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")
	file := createFile(config.instancesJsonFilePath, config, testState)
	writeToFile(file, `{"instances": [{"application_id": "123"}]}`, testState)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	instanceEvent, ok := <-instanceEventsChan
	if !ok {
		testState.Error("There was no instanceEvent!")
	}
	if instanceEvent.ApplicationId != "123" {
		testState.Error("Missing appid")
	}

	time.Sleep(1*time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	os.Truncate(config.instancesJsonFilePath, 0)
	writeToFile(file, `{"instances": [{"application_id": "123"}]}`, testState)

	select {
	case instanceEvent = <-instanceEventsChan:
		testState.Error("We just got an old instanceEvent, %s", instanceEvent)
	default:
		// OK
	}
}

func TestThatARemovedInstanceWillBeRemoved(testState *testing.T) {
	config := initializeConfig("/tmp/config.json")
	file := createFile(config.instancesJsonFilePath, config, testState)
	writeToFile(file, `{"instances": [{"application_id": "123"}]}`, testState)

	instanceEventsChan := WatchInstancesJsonFileForChanges(&config)

	time.Sleep(1*time.Nanosecond) // ensure that the go function starts before we add proper data to the json config


	instanceEvent, ok := <-instanceEventsChan
	if !ok {
		testState.Error("There was no instanceEvent!")
	}
	if instanceEvent.ApplicationId != "123" {
		testState.Error("Missing appid")
	}
}
