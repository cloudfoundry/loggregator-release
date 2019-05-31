package batching_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBatching(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Batching Suite")
}
