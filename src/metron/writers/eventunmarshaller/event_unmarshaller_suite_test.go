package eventunmarshaller_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEventUnmarshaller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EventUnmarshaller Suite")
}
