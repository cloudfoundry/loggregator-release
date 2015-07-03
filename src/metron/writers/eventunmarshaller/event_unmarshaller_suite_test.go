package eventunmarshaller_test

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

func TestEventUnmarshaller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Event Unmarshaller Suite")
}

var fakeEventEmitter = fake.NewFakeEventEmitter("doppler")
var metricBatcher *metricbatcher.MetricBatcher

var _ = BeforeSuite(func() {
	sender := metric_sender.NewMetricSender(fakeEventEmitter)
	metricBatcher = metricbatcher.New(sender, 100*time.Millisecond)
	metrics.Initialize(sender, metricBatcher)
})
