package domain_test

import (
	"deaagent/domain"
	"io/ioutil"
	"path"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func getJsonFromSampleFilePath(filename string) []byte {
	_, thisTestFilename, _, _ := runtime.Caller(0)
	json, _ := ioutil.ReadFile(path.Join(path.Dir(thisTestFilename), "..", "..", "..", "samples", filename))
	return json
}

var _ = Describe("ReadTasks", func() {
	It("reads tasks", func() {
		tasks, err := domain.ReadTasks(getJsonFromSampleFilePath("dea_instances.json"))

		Expect(err).NotTo(HaveOccurred())

		Expect(tasks).To(HaveLen(2))

		expectedInstanceTask := domain.Task{
			ApplicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
			Index:               0,
			WardenJobId:         272,
			WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1",
			SourceName:          "App",
		}

		expectedStagingTask := domain.Task{
			ApplicationId:       "23489sd0-f985-fjga-nsd1-sdg5lhd9nskh",
			WardenJobId:         355,
			WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo2",
			SourceName:          "STG",
		}

		Expect(tasks["/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272"]).To(Equal(expectedInstanceTask))
		Expect(tasks["/var/vcap/data/warden/depot/16vbs06ibo2/jobs/355"]).To(Equal(expectedStagingTask))
	})

	It("reads multiple tasks", func() {
		_, filename, _, _ := runtime.Caller(0)
		filepath := path.Join(path.Dir(filename), "..", "..", "..", "samples", "multi_instances.json")
		json, _ := ioutil.ReadFile(filepath)

		tasks, err := domain.ReadTasks(json)

		Expect(err).NotTo(HaveOccurred())

		Expect(tasks).To(HaveLen(6))

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

		Expect(applicationIds).To(ConsistOf(expectedApplicationIds))
	})

	It("reads multiple tasks with drain urls", func() {
		_, err := domain.ReadTasks(getJsonFromSampleFilePath("multi_instances.json"))
		Expect(err).NotTo(HaveOccurred())
	})

	It("reads starting tasks", func() {
		tasks, err := domain.ReadTasks(getJsonFromSampleFilePath("starting_instances.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(tasks).To(HaveLen(1))
		Expect(tasks).To(HaveKey("/var/vcap/data/warden/depot/345asndhaena/jobs/12"))
	})

	It("handles parsing empty data", func() {
		_, err := domain.ReadTasks(make([]byte, 0))
		Expect(err).To(HaveOccurred())

		_, err = domain.ReadTasks(make([]byte, 10))
		Expect(err).To(HaveOccurred())

		_, err = domain.ReadTasks(nil)
		Expect(err).To(HaveOccurred())
	})

	It("handles bad json", func() {
		_, err := domain.ReadTasks([]byte(`{ "instances" : [}`))
		Expect(err).To(HaveOccurred())
	})

	It("ignores non running instances", func() {
		tasks, err := domain.ReadTasks(getJsonFromSampleFilePath("instances.crashed.json"))

		Expect(err).ToNot(HaveOccurred())
		Expect(tasks).To(HaveLen(1))

		Expect(tasks).To(HaveKey("/var/vcap/data/warden/depot/170os7ali6q/jobs/15"))
		Expect(tasks).NotTo(HaveKey("/var/vcap/data/warden/depot/345asndhaena/jobs/12"))
	})

	It("ignores tasks without warden container path or job id", func() {
		tasks, err := domain.ReadTasks(getJsonFromSampleFilePath("instances.no_warden_path_or_id.json"))

		Expect(err).NotTo(HaveOccurred())
		Expect(tasks).To(BeEmpty())
	})

	It("ignores staging tasks without warden job id", func() {
		tasks, err := domain.ReadTasks(getJsonFromSampleFilePath("staging_tasks.no_job_id.json"))
		Expect(err).ToNot(HaveOccurred())
		Expect(tasks).To(HaveLen(1))

		Expect(tasks).To(HaveKey("/var/vcap/data/warden/depot/17fsdo7qper/jobs/49"), "Did not find staging task with warden job id.")
	})

	It("reads stopping tasks", func() {
		tasks, err := domain.ReadTasks(getJsonFromSampleFilePath("stopping_instances.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(tasks).To(HaveLen(1))
	})
})
