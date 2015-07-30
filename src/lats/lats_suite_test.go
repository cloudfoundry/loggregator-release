package lats_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/cf-test-helpers/helpers"
	"testing"
)

var config helpers.Config

func TestLats(t *testing.T) {
	RegisterFailHandler(Fail)

	var environment *helpers.Environment

	BeforeSuite(func() {
		config = helpers.LoadConfig()

		context := helpers.NewContext(config)
		environment = helpers.NewEnvironment(context)

		environment.Setup()
	})

	AfterSuite(func() {
		environment.Teardown()
	})

	RunSpecs(t, "Lats Suite")
}
