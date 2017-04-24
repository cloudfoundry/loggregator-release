package monitor_test

import (
	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"log"
	"testing"
)

func TestMonitor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Monitor Suite")
}

var (
	fakeEventEmitter *fake.FakeEventEmitter
)

var _ = BeforeSuite(func() {
	log.SetOutput(GinkgoWriter)
	fakeEventEmitter = fake.NewFakeEventEmitter("MonitorTest")
	sender := metric_sender.NewMetricSender(fakeEventEmitter)

	metrics.Initialize(sender, nil)
})

var _ = AfterSuite(func() {
	fakeEventEmitter.Close()
})
