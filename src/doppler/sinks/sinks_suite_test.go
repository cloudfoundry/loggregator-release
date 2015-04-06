package sinks_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSinks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sinks Suite")
}
