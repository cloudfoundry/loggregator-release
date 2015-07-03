package metrics_test

import (
	"time"

	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

var fakeEventEmitter = fake.NewFakeEventEmitter("doppler")

var _ = BeforeSuite(func() {
	sender := metric_sender.NewMetricSender(fakeEventEmitter)
	batcher := metricbatcher.New(sender, 100*time.Millisecond)
	metrics.Initialize(sender, batcher)
})
