package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTCPEchoServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TCP Echo Server Suite")
}
