package elector_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestElector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Elector Suite")
}
