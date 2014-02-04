package sinkserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSinkserver(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Sinkserver Suite")
}
