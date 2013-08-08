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
		spaceId:             "4aa9506e-277f-41ab-b764-a35c0b123456",
		organizationId:      "9876506e-277f-41ab-b764-a35c0b123456",
		wardenJobId:         272,
		wardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1",
		index:               0}

	assert.Equal(t, expectedInstance, instances["/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272"])
}

func TestReadingMultipleInstances(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "samples", "multi_instances.json")
	json, _ := ioutil.ReadFile(filepath)

	instances, err := readInstances(json)

	assert.NoError(t, err)

	assert.Equal(t, 2, len(instances))

	var applicationIds [2]string
	expectedApplicationIds := [2]string{
		"e0e12b41-78d4-43ff-a5ae-20422bedf22f",
		"a59ebe7a-002a-4530-8d69-8bf53bc845d5",
	}

	i := 0
	for _, instance := range instances {
		applicationIds[i] = instance.applicationId
		i++
	}

	assert.Equal(t, applicationIds, expectedApplicationIds)
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

func TestReadingInstancesIgnoresNonRunningInstances(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "samples", "instances.crashed.json")
	json, _ := ioutil.ReadFile(filepath)

	instances, err := readInstances(json)

	assert.NoError(t, err)

	assert.Equal(t, 1, len(instances))

	if _, ok := instances["/var/vcap/data/warden/depot/170os7ali6q/jobs/15"]; !ok {
		t.Errorf("Did not find active applicaiton.")
	}

	if _, ok := instances["/var/vcap/data/warden/depot/345asndhaena/jobs/12"]; ok {
		t.Errorf("Found crashed applicaiton.")
	}
}
