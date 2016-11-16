package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLinter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Linter Suite")
}
