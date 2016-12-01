package component_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestComponentTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ComponentTests Suite")
}
