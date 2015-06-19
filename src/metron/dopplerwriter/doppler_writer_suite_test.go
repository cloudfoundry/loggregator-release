package dopplerwriter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDopplerWriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Doppler Writer Suite")
}
