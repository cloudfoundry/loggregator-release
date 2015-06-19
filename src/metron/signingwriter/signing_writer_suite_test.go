package signingwriter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSigningWriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Signing Writer Suite")
}
