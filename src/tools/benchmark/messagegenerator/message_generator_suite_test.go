package messagegenerator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMessagegenerator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Messagegenerator Suite")
}
