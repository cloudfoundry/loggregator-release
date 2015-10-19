package dopplerforwarder_test

import (
	"metron/writers/dopplerforwarder"

	"github.com/cloudfoundry/loggregatorlib/loggertesthelper"
	"github.com/cloudfoundry/loggregatorlib/loggregatorclient"

	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/gosteno"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DopplerForwarder", func() {
	var (
		clientPool *mockClientPool
		logger     *gosteno.Logger
		forwarder  *dopplerforwarder.DopplerForwarder
	)

	BeforeEach(func() {
		clientPool = &mockClientPool{}
		logger = loggertesthelper.Logger()
		forwarder = dopplerforwarder.New(clientPool, logger)
	})

	It("sends messages to a random doppler", func() {
		message := []byte("Some message")
		forwarder.Write(message)

		Expect(clientPool.randomClient).ToNot(BeNil())

		data := clientPool.randomClient.data
		Expect(data).To(HaveLen(1))
		Expect(data[0]).To(Equal(message))
	})

	It("sends a metric for the number of sent messages", func() {
		sender := fake.NewFakeMetricSender()
		metrics.Initialize(sender, metricbatcher.New(sender, time.Millisecond*10))

		message := []byte("Some message")
		forwarder.Write(message)

		Eventually(func() uint64 { return sender.GetCounter("DopplerForwarder.sentMessages") }).Should(BeEquivalentTo(1))
	})
})

type mockClientPool struct {
	randomClient *mockClient
}

func (m *mockClientPool) RandomClient() (loggregatorclient.Client, error) {
	m.randomClient = &mockClient{}
	return m.randomClient, nil
}

type mockClient struct {
	data [][]byte
}

func (m *mockClient) Scheme() string {
	return "mock"
}

func (m *mockClient) Address() string {
	return ""
}

func (m *mockClient) Send(p []byte) {
	m.data = append(m.data, p)
}

func (m *mockClient) Stop() {

}
