package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRlp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RLP Suite")
}
