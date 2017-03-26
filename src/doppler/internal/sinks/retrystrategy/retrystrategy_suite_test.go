package retrystrategy_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRetrystrategy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Retrystrategy Suite")
}
