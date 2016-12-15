package legacyclientpool_test

import (
	"io/ioutil"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLegacyclientpool(t *testing.T) {
	if !testing.Verbose() {
		log.SetOutput(ioutil.Discard)
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "Legacyclientpool Suite")
}
