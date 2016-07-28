package logcounter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogcounter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logcounter Suite")
}
