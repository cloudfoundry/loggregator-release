package truncatingbuffer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTruncatingbuffer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TruncatingBuffer Suite")
}
