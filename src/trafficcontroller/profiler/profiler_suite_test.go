package profiler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestProfiler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Profiler Suite")
}
