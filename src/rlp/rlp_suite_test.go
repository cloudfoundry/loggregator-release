package main_test

import (
	"metric"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRlp(t *testing.T) {
	metric.Setup()
	RegisterFailHandler(Fail)
	RunSpecs(t, "RLP compile main")
}
