package dea_logging_agent

import (
	"testing"
	"os"
	"time"
)

func initializeConfig() Config {
	config := Config{instancesJsonFilePath: "/tmp/config.json"}
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

func TestThatFunctionExistsWhenFileCantBeOpened(testState *testing.T) {
	config := initializeConfig()

	instancesChan := WatchInstancesJsonFileForChanges(&config)

	if _, ok := <-instancesChan; ok {
		testState.Error("Found an instance, but should have died")
	}
}

func TestThatAnExistinginstanceWillBeSeen(testState *testing.T) {
	config := initializeConfig()
	file := createFile(config.instancesJsonFilePath, config, testState)
	writeToFile(file, `{"Instances": [{"AppId": "123"}]}`, testState)

	instancesChan := WatchInstancesJsonFileForChanges(&config)

	if instance := <-instancesChan; instance.AppId != "123" {
		testState.Error("Missing appid")
	}
}

func TestThatANewinstanceWillBeSeen(testState *testing.T) {
	config := initializeConfig()
	file := createFile(config.instancesJsonFilePath, config, testState)

	instancesChan := WatchInstancesJsonFileForChanges(&config)

	time.Sleep(1*time.Nanosecond) // ensure that the go function starts before we add proper data to the json config

	writeToFile(file, `{"Instances": [{"AppId": "123"}]}`, testState)

	instance, ok := <-instancesChan
	if !ok {
		testState.Error("There was no instance!")
	}
	if instance.AppId != "123" {
		testState.Error("Missing appid")
	}
}

func TestThatOnlyNewInstancesWillBeSeen(testState *testing.T) {
	config := initializeConfig()
	file := createFile(config.instancesJsonFilePath, config, testState)
	writeToFile(file, `{"Instances": [{"AppId": "123"}]}`, testState)

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
	writeToFile(file, `{"Instances": [{"AppId": "123"}]}`, testState)

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
