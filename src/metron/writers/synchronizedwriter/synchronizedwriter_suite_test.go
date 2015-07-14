package synchronizedwriter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSynchronizedwriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Synchronizedwriter Suite")
}
