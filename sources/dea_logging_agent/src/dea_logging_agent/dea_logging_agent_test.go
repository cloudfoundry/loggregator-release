package dea_logging_agent

import (
	"testing"
	"os"
)

func TestThatANewInstanceWillBeMonitored(testState *testing.T) {
	os.Remove("/tmp/config.json")

	file, error := os.Create("/tmp/config.json")
	if error != nil {
		testState.Error(error)
	}

	_, error = file.WriteString(`{"Instances": [{"AppId": "123"}]}`)
	if error != nil {
		testState.Error(error)
	}

	config := Config{ instancesJsonFilePath: "/tmp/config.json"}
	instancesChan := WatchInstancesJsonFileForChanges(&config)

	instance := <-instancesChan

	if instance.AppId != "123" {
		testState.Error("Missing appid")
	}
}

func TestThatARemovedInstanceWillBeRemovedFromMonitoring(testState *testing.T) {
	testState.Skip()
}
