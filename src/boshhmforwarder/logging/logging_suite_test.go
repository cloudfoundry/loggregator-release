package logging_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogging(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bosh HM Forwarder - Logging Suite")
}
