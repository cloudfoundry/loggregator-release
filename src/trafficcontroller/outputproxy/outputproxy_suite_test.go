package outputproxy_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOutputProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OutputProxy Suite")
}
