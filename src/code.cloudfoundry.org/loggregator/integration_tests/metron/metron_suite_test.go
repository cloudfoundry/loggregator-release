package metron_test

import (
	"testing"

	"code.cloudfoundry.org/loggregator/integration_tests/binaries"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMetron(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metron Integration Test Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	bp, errors := binaries.Build()
	for err := range errors {
		Expect(err).ToNot(HaveOccurred())
	}
	text, err := bp.Marshal()
	Expect(err).ToNot(HaveOccurred())
	return text
}, func(bpText []byte) {
	var bp binaries.BuildPaths
	err := bp.Unmarshal(bpText)
	Expect(err).ToNot(HaveOccurred())
	bp.SetEnv()
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	binaries.Cleanup()
})
