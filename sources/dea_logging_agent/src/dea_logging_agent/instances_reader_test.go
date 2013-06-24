package dea_logging_agent

import (
	"testing"
	"runtime"
	"path"
	"io/ioutil"
)

func TestReadingInstances(testState *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "samples", "dea_instances.json")
	json, _ := ioutil.ReadFile(filepath)

	instances, err := ReadInstances(json)

	if err != nil {
		testState.Error(err)
	}
	if len(instances) != 1 {
		testState.Errorf("There should be one instance, but there are %v", len(instances))
	}

	expectedInstance := Instance{
		ApplicationId: "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		WardenJobId: 272,
		WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}
	if instances[0] != expectedInstance {
		testState.Errorf("Expected appid to be %v but was %v", expectedInstance, instances[0])
	}
}

