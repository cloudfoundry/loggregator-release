package legacy_message_converter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLegacyMessageConverter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LegacyMessageConverter Suite")
}
