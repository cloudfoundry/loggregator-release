package component_test

import (
	"log"
	"testing"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"integration_tests/binaries"
)

func TestComponentTests(t *testing.T) {
	grpclog.SetLogger(log.New(GinkgoWriter, "", log.LstdFlags))
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metron ComponentTests Suite")
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
