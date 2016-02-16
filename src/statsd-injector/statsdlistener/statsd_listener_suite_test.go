package statsdlistener_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStatsdlistener(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Statsdlistener Suite")
}
