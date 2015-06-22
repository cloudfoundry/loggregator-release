package varzforwarder_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVarzForwarder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VarzForwarder Suite")
}
