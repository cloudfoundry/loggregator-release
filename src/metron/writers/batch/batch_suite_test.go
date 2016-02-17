package batch_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Batch Suite")
}
