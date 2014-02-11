package blacklist_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBlacklist(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Blacklist Suite")
}
