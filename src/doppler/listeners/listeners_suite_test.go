package listeners_test

import (
	"github.com/cloudfoundry/dropsonde/emitter/fake"
	"github.com/cloudfoundry/dropsonde/metric_sender"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"crypto/tls"
	"io/ioutil"
	"testing"
	"time"
)

func TestTcplistener(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Listeners Suite")
}

var tlsconfig *tls.Config
var fakeEventEmitter = fake.NewFakeEventEmitter("doppler")
var metricBatcher *metricbatcher.MetricBatcher

var _ = BeforeSuite(func() {
	certContent, err := ioutil.ReadFile("fixtures/key.crt")
	if err != nil {
		panic(err)
	}

	keyContent, err := ioutil.ReadFile("fixtures/key.key")
	if err != nil {
		panic(err)
	}

	cert, err := tls.X509KeyPair(certContent, keyContent)
	if err != nil {
		panic(err)
	}

	tlsconfig = &tls.Config{
		Certificates:           []tls.Certificate{cert},
		SessionTicketsDisabled: true,
	}

	sender := metric_sender.NewMetricSender(fakeEventEmitter)
	metricBatcher = metricbatcher.New(sender, 100*time.Millisecond)
	metrics.Initialize(sender, metricBatcher)
})
