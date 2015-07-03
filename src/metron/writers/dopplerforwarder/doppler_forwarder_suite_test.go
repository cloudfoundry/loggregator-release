package dopplerforwarder_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDopplerForwarder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Doppler Forwarder Suite")
}
