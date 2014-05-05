package hasher_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHasher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hasher Suite")
}
