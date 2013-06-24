package dea_logging_agent

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"runtime"
	"path"
	"io/ioutil"
)

func TestReadingInstances(testState *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "samples", "dea_instances.json")
	json, _ := ioutil.ReadFile(filepath)

	instances, err := ReadInstances(json)

	assert.NoError(testState, err)

	assert.Equal(testState, 1, len(instances))

	expectedInstance := Instance{
		ApplicationId: "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		WardenJobId: 272,
		WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}
	assert.Equal(testState, expectedInstance, instances[0])
}

