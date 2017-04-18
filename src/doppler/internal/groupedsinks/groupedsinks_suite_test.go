package groupedsinks_test

import (
	"log"
	"metric"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGroupedsinks(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	metric.Setup()
	RegisterFailHandler(Fail)
	RunSpecs(t, "GroupedSinks Suite")
}
