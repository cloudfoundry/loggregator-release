package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogCounterApp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Log Counter App Suite")
}
