package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHTTPEchoServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HTTP Echo Server Suite")
}
