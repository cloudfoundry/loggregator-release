package serveraddressprovider_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestServeraddressprovider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Serveraddressprovider Suite")
}
