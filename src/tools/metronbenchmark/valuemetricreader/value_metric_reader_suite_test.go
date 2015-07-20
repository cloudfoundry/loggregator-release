package valuemetricreader_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestValueMetricreader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ValueMetricReader Suite")
}
