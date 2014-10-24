package pingsender_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPingSender(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PingSender Suite")
}
