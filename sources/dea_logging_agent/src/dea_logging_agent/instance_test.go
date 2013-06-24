package dea_logging_agent

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestIdentifier(testState *testing.T) {
	instance := Instance{
		ApplicationId: "4aa9506e-277f-41ab-b764-a35c0b96fa1b",
		WardenJobId: "272",
		WardenContainerPath: "/var/vcap/data/warden/depot/16vbs06ibo1"}

	assert.Equal(testState, "/var/vcap/data/warden/depot/16vbs06ibo1/jobs/272", instance.Identifier())
}
