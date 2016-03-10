package eventtypereader_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEventtypereader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eventtypereader Suite")
}
