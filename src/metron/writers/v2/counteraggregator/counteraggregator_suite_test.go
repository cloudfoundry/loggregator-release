package counteraggregator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCounteraggregator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Counteraggregator Suite")
}
