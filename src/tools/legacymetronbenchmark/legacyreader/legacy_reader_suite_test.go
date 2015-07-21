package legacyreader_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLegacyReader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LegacyReader Suite")
}
