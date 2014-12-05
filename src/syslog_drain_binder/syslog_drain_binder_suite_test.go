package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSyslogDrainBinder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SyslogDrainBinder Suite")
}
