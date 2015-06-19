package marshallingeventwriter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMarshallingEventWriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Marshalling Event Writer Suite")
}
