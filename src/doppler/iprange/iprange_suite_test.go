package iprange_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestIprange(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IPRange Suite")
}
