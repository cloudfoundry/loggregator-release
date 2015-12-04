package sinkmanager_test

import (
	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSinkmanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sinkmanager Suite")
}

var fakeMetricSender = fake.NewFakeMetricSender()

var _ = BeforeSuite(func() {
	batcher := metricbatcher.New(fakeMetricSender, 1*time.Millisecond)
	metrics.Initialize(fakeMetricSender, batcher)
})
