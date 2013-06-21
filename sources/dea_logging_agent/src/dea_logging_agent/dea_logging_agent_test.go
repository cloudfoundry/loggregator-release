package dea_logging_agent

import (
	"testing"
	"os"
	"time"
)

func TestThatFunctionExistsWhenFileCantBeOpened(testState *testing.T) {
	config := Config{instancesJsonFilePath: "/tmp/config.json"}
	os.Remove(config.instancesJsonFilePath)

	instancesChan := WatchInstancesJsonFileForChanges(&config)

	if _, ok := <-instancesChan; ok {
		testState.Error("Found an instance, but should have died")
	}
}

func TestThatAnExistinginstanceWillBeSeen(testState *testing.T) {
	config := Config{instancesJsonFilePath: "/tmp/config.json"}
	os.Remove(config.instancesJsonFilePath)

	file, error := os.Create(config.instancesJsonFilePath)
	if error != nil {
		testState.Error(error)
	}
	_, error = file.WriteString(`{"Instances": [{"AppId": "123"}]}`)
	if error != nil {
		testState.Error(error)
	}

	instancesChan := WatchInstancesJsonFileForChanges(&config)

	if instance := <-instancesChan; instance.AppId != "123" {
		testState.Error("Missing appid")
	}
}

func TestThatANewinstanceWillBeSeen(testState *testing.T) {
	config := Config{instancesJsonFilePath: "/tmp/config.json"}
	os.Remove(config.instancesJsonFilePath)

	file, error := os.Create(config.instancesJsonFilePath)
	if error != nil {
		testState.Error(error)
	}

	instancesChan := WatchInstancesJsonFileForChanges(&config)

	time.Sleep(1*time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	_, error = file.WriteString(`{"Instances": [{"AppId": "123"}]}`)
	if error != nil {
		testState.Error(error)
	}

	instance, ok := <-instancesChan
	if !ok {
		testState.Error("There was no instance!")
	}
	if instance.AppId != "123" {
		testState.Error("Missing appid")
	}
}

func TestThatOnlyNewInstancesWillBeSeen(testState *testing.T) {
	config := Config{instancesJsonFilePath: "/tmp/config.json"}
	os.Remove(config.instancesJsonFilePath)

	file, error := os.Create(config.instancesJsonFilePath)
	if error != nil {
		testState.Error(error)
	}
	_, error = file.WriteString(`{"Instances": [{"AppId": "123"}]}`)
	if error != nil {
		testState.Error(error)
	}

	instancesChan := WatchInstancesJsonFileForChanges(&config)

	instance, ok := <-instancesChan
	if !ok {
		testState.Error("There was no instance!")
	}
	if instance.AppId != "123" {
		testState.Error("Missing appid")
	}

	time.Sleep(1*time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	os.Truncate(config.instancesJsonFilePath, 0)
	_, error = file.WriteString(`{"Instances": [{"AppId": "123"}]}`)
	if error != nil {
		testState.Error(error)
	}

	select {
	case instance = <-instancesChan:
		testState.Error("We just got an old instance, %s", instance)
	default:
		// OK
	}
}

func TestThatARemovedInstanceWillBeRemoved(testState *testing.T) {
	testState.Skip()
}
