package domain_test

import (
	"deaagent/domain"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task", func() {
	It("sets identifier correctly", func() {
		task := domain.Task{
			ApplicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
			WardenJobId:         272,
			WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}

		Expect(task.Identifier()).To(Equal("/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272"))
	})
})
