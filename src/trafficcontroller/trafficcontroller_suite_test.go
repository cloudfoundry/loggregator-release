package main_test

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTrafficcontroller(t *testing.T) {
	cfcomponent.DefaultYagnatsClientProvider = func(logger *gosteno.Logger, c *cfcomponent.Config) (yagnats.NATSClient, error) {
		return nil, nil
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Trafficcontroller Suite")
}
