package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMetronBenchmark(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metron Benchmark Suite")
}
