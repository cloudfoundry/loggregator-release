package eventmarshaller_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEventMarshaller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EventMarhsaller Suite")
}
