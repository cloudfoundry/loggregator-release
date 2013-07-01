package deaagent

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

	instances, err := readInstances(json)

	assert.NoError(t, err)

	assert.Equal(t, 1, len(instances))

	expectedInstance := instance{
		applicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		wardenJobId:         272,
		wardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1",
		index:               0}

	assert.Equal(t, expectedInstance, instances["/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272"])
}

func TestErrorHandlingWhenParsingEmptyData(t *testing.T) {
	_, err := readInstances(make([]byte, 0))
	assert.Error(t, err)

	_, err = readInstances(make([]byte, 10))
	assert.Error(t, err)

	_, err = readInstances(nil)
	assert.Error(t, err)
}

func TestErrorHandlingWithBadJson(t *testing.T) {
	_, err := readInstances([]byte(`{ "instances" : [}`))
	assert.Error(t, err)
}
