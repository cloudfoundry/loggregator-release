package picker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPicker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Picker Suite")
}
