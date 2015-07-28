package lats_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLats(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lats Suite")
}
