package accesslogger_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAccesslogger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Accesslogger Suite")
}
