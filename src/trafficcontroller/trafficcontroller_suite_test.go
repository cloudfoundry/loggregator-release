package trafficcontroller_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTrafficcontroller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Trafficcontroller Suite")
}
