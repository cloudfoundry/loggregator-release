package dopplerservice_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAnnouncer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DopplerService Suite")
}
