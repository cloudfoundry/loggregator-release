package envelopewrapper_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEnvelopewrapper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Envelopewrapper Suite")
}
