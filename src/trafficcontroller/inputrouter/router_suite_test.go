package inputrouter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestInputRouter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "InputRouter Suite")
}
