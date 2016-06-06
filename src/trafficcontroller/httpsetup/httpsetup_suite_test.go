package httpsetup_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHttpsetup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Httpsetup Suite")
}
