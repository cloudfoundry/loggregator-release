package containermetric_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestContainermetric(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ContainerMetricSink Suite")
}
