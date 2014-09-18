package legacyproxy_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLegacyproxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Legacyproxy Suite")
}
