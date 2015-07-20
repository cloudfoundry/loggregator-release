package logmessagereader_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLogMessageReader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LogMessageReader Suite")
}
