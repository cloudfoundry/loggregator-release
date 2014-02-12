package deaagent_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDeaagent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Deaagent Suite")
}
