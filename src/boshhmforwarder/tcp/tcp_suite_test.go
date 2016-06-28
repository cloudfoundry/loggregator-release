package tcp_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTcp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bosh HM Forwarder - TCP Suite")
}
