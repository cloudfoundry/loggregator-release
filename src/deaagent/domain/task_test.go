package domain_test

import (
	"deaagent/domain"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIdentifier(t *testing.T) {
	task := domain.Task{
		ApplicationId:       "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		WardenJobId:         272,
		WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}

	assert.Equal(t, "/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272", task.Identifier())
}
