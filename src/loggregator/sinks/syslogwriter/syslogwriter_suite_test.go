package syslogwriter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSyslogwriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Syslogwriter Suite")
}
