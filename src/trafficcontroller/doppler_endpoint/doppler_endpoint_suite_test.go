package doppler_endpoint_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDopplerEndpoint(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DopplerEndpoint Suite")
}
