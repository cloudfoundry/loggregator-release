package tagger_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTagger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tagger Suite")
}
