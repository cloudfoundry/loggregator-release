package dopplerservice_test

import (
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAnnouncer(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)

	// Using etcd for service discovery is a deprecated code path
	// RunSpecs(t, "DopplerService Suite")
}
