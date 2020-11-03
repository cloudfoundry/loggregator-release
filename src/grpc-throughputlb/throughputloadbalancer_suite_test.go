package throughputlb_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestThroughputloadbalancer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Throughputloadbalancer Suite")
}
