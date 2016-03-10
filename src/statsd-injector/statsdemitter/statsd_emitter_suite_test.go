package statsdemitter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStatsdemitter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Statsdemitter Suite")
}
