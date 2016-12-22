package sinkmanager_test

import (
	"log"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	fakeMS "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSinkmanager(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sinkmanager Suite")
}

var (
	fakeMetricSender *fakeMS.FakeMetricSender
	fakeEventEmitter *fake.FakeEventEmitter
)

var _ = BeforeSuite(func() {
	fakeEventEmitter = fake.NewFakeEventEmitter("doppler")
	fakeMetricSender = fakeMS.NewFakeMetricSender()
	metrics.Initialize(fakeMetricSender, nil)
})
