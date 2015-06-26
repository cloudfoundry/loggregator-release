package messagewriter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMessagewriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Messagewriter Suite")
}
