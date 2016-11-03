package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogSpinner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Log Spinner Suite")
}
