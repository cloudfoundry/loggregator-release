package marshaller_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMarshaller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Marshaller Suite")
}
