package sinks_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestContainermetric(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ContainerMetricSink Suite")
}
