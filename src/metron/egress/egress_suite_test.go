package egress_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEgress(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Egress Suite")
}
