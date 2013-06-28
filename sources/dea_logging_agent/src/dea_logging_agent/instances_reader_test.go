package dea_logging_agent

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path"
	"runtime"
	"testing"
)

func TestReadingInstances(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "samples", "dea_instances.json")
	json, _ := ioutil.ReadFile(filepath)

	instances, err := ReadInstances(json)

	assert.NoError(t, err)

	assert.Equal(t, 1, len(instances))

	expectedInstance := Instance{
		ApplicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		WardenJobId:         272,
		WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1",
		Index:               0}

	assert.Equal(t, expectedInstance, instances["/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272"])
}

func TestErrorHandlingWhenParsingEmptyData(t *testing.T) {
	_, err := ReadInstances(make([]byte, 0))
	assert.Error(t, err)

	_, err = ReadInstances(make([]byte, 10))
	assert.Error(t, err)

	_, err = ReadInstances(nil)
	assert.Error(t, err)
}

func TestErrorHandlingWithBadJson(t *testing.T) {
	_, err := ReadInstances([]byte(`{ "instances" : [}`))
	assert.Error(t, err)
}
