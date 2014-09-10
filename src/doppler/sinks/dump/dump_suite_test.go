package dump_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDump(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DumpSink Suite")
}
