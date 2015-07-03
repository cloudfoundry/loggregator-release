package metricsreporter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/onsi/gomega/gexec"
)

func TestMetricsreporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metricsreporter Suite")
}

var pathToMetricsReporter string

var _ = BeforeSuite(func() {
	var err error
	pathToMetricsReporter, err = gexec.Build("tools/metronbenchmark/metricsreporter")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
