package sinkmanager_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSinkmanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sinkmanager Suite")
}
