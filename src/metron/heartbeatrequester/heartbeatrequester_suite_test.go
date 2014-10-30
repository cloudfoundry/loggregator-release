package heartbeatrequester_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHeartbeatRequester(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HeartbeatRequester Suite")
}
