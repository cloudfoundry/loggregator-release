package cloud_controller_poller_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCloudControllerPoller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudControllerPoller Suite")
}
