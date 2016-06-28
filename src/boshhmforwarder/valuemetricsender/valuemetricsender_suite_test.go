package valuemetricsender_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestValuemetricsender(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bosh HM Forwarder - Value Metric Sender Suite")
}
