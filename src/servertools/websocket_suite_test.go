package servertools_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestServertools(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Servertools Suite")
}
