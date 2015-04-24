package messageaggregator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMessageAggregator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MessageAggregator Suite")
}
