package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEnvelopeEmitter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Envelope Emitter Suite")
}
