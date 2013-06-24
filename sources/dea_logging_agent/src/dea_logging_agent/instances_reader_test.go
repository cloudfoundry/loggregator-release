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
	if instances[0].ApplicationId != "4aa9506e-277f-41ab-b764-a35c0b96fa1b" {
		testState.Errorf("Expected appid to be '4aa9506e-277f-41ab-b764-a35c0b96fa1b' but was %v", instances[0].ApplicationId)
	}
}

