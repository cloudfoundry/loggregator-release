package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStatsdGoClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Statsd Go Client Suite")
}
