package networkreader_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestNetworkReader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NetworkReader Suite")
}
