package metricsreporter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMetricsreporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metricsreporter Suite")
}
