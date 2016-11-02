package plumbing_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPlumbing(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plumbing Suite")
}
