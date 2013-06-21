package dea_logging_agent

import (
	"testing"
	"os"
	"time"
)

func TestThatFunctionExistsWhenFileCantBeOpened(testState *testing.T) {
	os.Remove("/tmp/config.json")

	config := Config{instancesJsonFilePath: "/tmp/config.json"}
	instancesChan := WatchInstancesJsonFileForChanges(&config)

	if _, ok := <-instancesChan; ok {
		testState.Error("Found an instance, but should have died")
	}
}

func TestThatAnExistingInstanceWillBeSeen(testState *testing.T) {
	os.Remove("/tmp/config.json")

	file, error := os.Create("/tmp/config.json")
	if error != nil {
		testState.Error(error)
	}

	_, error = file.WriteString(`{"Instances": [{"AppId": "123"}]}`)
	if error != nil {
		testState.Error(error)
	}

	config := Config{instancesJsonFilePath: "/tmp/config.json"}
	instancesChan := WatchInstancesJsonFileForChanges(&config)

	if instance := <-instancesChan; instance.AppId != "123" {
		testState.Error("Missing appid")
	}
}

func TestThatANewInstanceWillBeSeen(testState *testing.T) {
	os.Remove("/tmp/config.json")

	file, error := os.Create("/tmp/config.json")
	if error != nil {
		testState.Error(error)
	}

	config := Config{instancesJsonFilePath: "/tmp/config.json"}
	instancesChan := WatchInstancesJsonFileForChanges(&config)

	time.Sleep(1* time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

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

func TestThatARemovedInstanceWillBeRemoved(testState *testing.T) {
	testState.Skip()
}
