package eventwriter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEventWriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EventWriter Suite")
}
