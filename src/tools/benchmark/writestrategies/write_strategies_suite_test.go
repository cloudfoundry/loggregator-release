package writestrategies_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestWriteStrategies(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WriteStrategies Suite")
}
