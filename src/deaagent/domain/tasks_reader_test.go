package domain_test

import (
	"deaagent/domain"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path"
	"runtime"
	"testing"
)

func TestReadingTask(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "..", "samples", "dea_instances.json")
	json, _ := ioutil.ReadFile(filepath)

	tasks, err := domain.ReadTasks(json)

	assert.NoError(t, err)

	assert.Equal(t, 2, len(tasks))

	expectedInstanceTask := domain.Task{
		ApplicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		Index:               0,
		WardenJobId:         272,
		WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1",
		SourceName:          "App",
		DrainUrls:           []string{}}

	expectedStagingTask := domain.Task{
		ApplicationId:       "23489sd0-f985-fjga-nsd1-sdg5lhd9nskh",
		WardenJobId:         355,
		WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo2",
		SourceName:          "STG",
		DrainUrls:           []string{}}

	assert.Equal(t, expectedInstanceTask, tasks["/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272"])
	assert.Equal(t, expectedStagingTask, tasks["/var/vcap/data/warden/depot/16vbs06ibo2/jobs/355"])
}

func TestReadingMultipleTasks(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "..", "samples", "multi_instances.json")
	json, _ := ioutil.ReadFile(filepath)

	tasks, err := domain.ReadTasks(json)

	assert.NoError(t, err)

	assert.Equal(t, 6, len(tasks))

	var applicationIds [6]string
	expectedApplicationIds := [6]string{
		"e0e12b41-78d4-43ff-a5ae-20422bedf22f",
		"d8df836e-e27d-45d4-a890-b2ce899788a4",
		"a59ebe7a-002a-4530-8d69-8bf53bc845d5",
		"cc01f618-6b62-428e-96c6-bc743dd235cd",
		"09fbc153-e2ed-43b3-8f42-b270d1937ec6",
		"243a34f-0724-4d9c-a0de-a0811cbabce6",
	}

	i := 0
	for _, task := range tasks {
		applicationIds[i] = task.ApplicationId
		i++
	}

	assert.Equal(t, applicationIds, expectedApplicationIds)
}

func TestReadingMultipleTasksWithDrainUrls(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "..", "samples", "multi_instances.json")
	json, _ := ioutil.ReadFile(filepath)

	tasks, err := domain.ReadTasks(json)

	assert.NoError(t, err)

	assert.Nil(t, tasks["/var/vcap/data/warden/depot/170os7ali6q/jobs/15"].DrainUrls)
	assert.Equal(t, tasks["/var/vcap/data/warden/depot/123ajkljfa/jobs/13"].DrainUrls, []string{})
	assert.Equal(t, tasks["/var/vcap/data/warden/depot/345asndhaena/jobs/12"].DrainUrls, []string{"syslog://10.20.30.40:8050"})

	assert.Nil(t, tasks["/var/vcap/data/warden/depot/17fsdo7qpeq/jobs/46"].DrainUrls)
	assert.Equal(t, tasks["/var/vcap/data/warden/depot/17fsdo7qper/jobs/49"].DrainUrls, []string{})
	assert.Equal(t, tasks["/var/vcap/data/warden/depot/17fsdo7qpes/jobs/56"].DrainUrls, []string{"syslog://10.20.30.40:8050"})
}

func TestReadingStartingTasks(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "..", "samples", "starting_instances.json")
	json, _ := ioutil.ReadFile(filepath)

	tasks, err := domain.ReadTasks(json)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(tasks))

	if _, ok := tasks["/var/vcap/data/warden/depot/345asndhaena/jobs/12"]; !ok {
		t.Errorf("Did not find starting application.")
	}
}

func TestErrorHandlingWhenParsingEmptyData(t *testing.T) {
	_, err := domain.ReadTasks(make([]byte, 0))
	assert.Error(t, err)

	_, err = domain.ReadTasks(make([]byte, 10))
	assert.Error(t, err)

	_, err = domain.ReadTasks(nil)
	assert.Error(t, err)
}

func TestErrorHandlingWithBadJson(t *testing.T) {
	_, err := domain.ReadTasks([]byte(`{ "instances" : [}`))
	assert.Error(t, err)
}

func TestReadingTasksIgnoresNonRunningInstances(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "..", "samples", "instances.crashed.json")
	json, _ := ioutil.ReadFile(filepath)

	tasks, err := domain.ReadTasks(json)

	assert.NoError(t, err)

	assert.Equal(t, 1, len(tasks))

	if _, ok := tasks["/var/vcap/data/warden/depot/170os7ali6q/jobs/15"]; !ok {
		t.Errorf("Did not find active applicaiton.")
	}

	if _, ok := tasks["/var/vcap/data/warden/depot/345asndhaena/jobs/12"]; ok {
		t.Errorf("Found crashed applicaiton.")
	}
}

func TestReadingTasksIgnoreStagingTasksWithoutWardenJobId(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	filepath := path.Join(path.Dir(filename), "..", "..", "..", "samples", "staging_tasks.no_job_id.json")
	json, _ := ioutil.ReadFile(filepath)

	tasks, err := domain.ReadTasks(json)

	assert.NoError(t, err)

	assert.Equal(t, 1, len(tasks))

	if _, ok := tasks["/var/vcap/data/warden/depot/17fsdo7qper/jobs/49"]; !ok {
		t.Errorf("Did not find staging task with warden job id.")
	}
}
