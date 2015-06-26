package experiment_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestExperiment(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Experiment Suite")
}
