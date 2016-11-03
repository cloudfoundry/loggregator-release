package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTSDBMessageSpewer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TSDB Message Spewer Suite")
}
