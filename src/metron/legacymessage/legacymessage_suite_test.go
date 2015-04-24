package legacymessage_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLegacyMessage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LegacyMessage Suite")
}
