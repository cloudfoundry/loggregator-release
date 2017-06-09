package v2_test

import (
	"io"
	"sync"

	"code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v2"
	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	plumbing "code.cloudfoundry.org/loggregator/plumbing/v2"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ingress", func() {
	var (
		mockDataSetter  *mockDataSetter
		mockSender      *mockDopplerIngress_SenderServer
		mockBatchSender *mockBatcherSenderServer
		healthRegistrar *SpyHealthRegistrar

		ingestor *v2.IngressServer
	)

	BeforeEach(func() {
		mockDataSetter = newMockDataSetter()
		mockSender = newMockDopplerIngress_SenderServer()
		mockBatchSender = newMockBatcherSenderServer()
		healthRegistrar = newSpyHealthRegistrar()

		ingestor = v2.NewIngressServer(
			mockDataSetter,
			SpyBatcher{},
			testhelper.NewMetricClient(),
			healthRegistrar,
		)
	})

	It("writes batches to the data setter", func() {
		mockBatchSender.RecvOutput.Ret0 <- &plumbing.EnvelopeBatch{
			Batch: []*plumbing.Envelope{
				{
					Message: &plumbing.Envelope_Log{
						Log: &plumbing.Log{
							Payload: []byte("hello-1"),
						},
					},
				},
				{
					Message: &plumbing.Envelope_Log{
						Log: &plumbing.Log{
							Payload: []byte("hello-2"),
						},
					},
				},
			},
		}

		mockBatchSender.RecvOutput.Ret1 <- nil
		mockBatchSender.RecvOutput.Ret0 <- nil
		mockBatchSender.RecvOutput.Ret1 <- io.EOF

		ingestor.BatchSender(mockBatchSender)
		Expect(mockDataSetter.SetCalled).To(HaveLen(2))
	})

	It("writes the v2 envelope as a v1 envelope to data setter", func() {
		mockSender.RecvOutput.Ret0 <- &plumbing.Envelope{
			Message: &plumbing.Envelope_Log{
				Log: &plumbing.Log{
					Payload: []byte("hello"),
				},
			},
		}
		mockSender.RecvOutput.Ret1 <- nil
		mockSender.RecvOutput.Ret0 <- nil
		mockSender.RecvOutput.Ret1 <- io.EOF

		ingestor.Sender(mockSender)
		Expect(mockDataSetter.SetCalled).To(HaveLen(1))
	})

	It("throws invalid envelopes on the ground", func() {
		mockSender.RecvOutput.Ret0 <- &plumbing.Envelope{}
		mockSender.RecvOutput.Ret1 <- nil
		mockSender.RecvOutput.Ret0 <- nil
		mockSender.RecvOutput.Ret1 <- io.EOF

		ingestor.Sender(mockSender)
		Expect(mockDataSetter.SetCalled).To(HaveLen(0))
	})

	Describe("health monitoring", func() {
		Describe("Sender()", func() {
			It("increments and decrements the number of ingress streams", func() {
				go ingestor.Sender(mockSender)

				Eventually(func() float64 {
					return healthRegistrar.Get("ingressStreamCount")
				}).Should(Equal(1.0))

				mockSender.RecvOutput.Ret0 <- nil
				mockSender.RecvOutput.Ret1 <- io.EOF

				Eventually(func() float64 {
					return healthRegistrar.Get("ingressStreamCount")
				}).Should(Equal(0.0))
			})
		})

		Describe("BatchSender()", func() {
			It("increments and decrements the number of ingress streams", func() {
				go ingestor.BatchSender(mockBatchSender)

				Eventually(func() float64 {
					return healthRegistrar.Get("ingressStreamCount")
				}).Should(Equal(1.0))

				mockBatchSender.RecvOutput.Ret0 <- nil
				mockBatchSender.RecvOutput.Ret1 <- io.EOF

				Eventually(func() float64 {
					return healthRegistrar.Get("ingressStreamCount")
				}).Should(Equal(0.0))
			})
		})
	})
})

type SpyBatcher struct {
	metricbatcher.BatchCounterChainer
}

func (s SpyBatcher) BatchCounter(string) metricbatcher.BatchCounterChainer {
	return s
}

func (s SpyBatcher) Increment() {
}

type SpyHealthRegistrar struct {
	mu     sync.Mutex
	values map[string]float64
}

func newSpyHealthRegistrar() *SpyHealthRegistrar {
	return &SpyHealthRegistrar{
		values: make(map[string]float64),
	}
}

func (s *SpyHealthRegistrar) Inc(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name]++
}

func (s *SpyHealthRegistrar) Dec(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name]--
}

func (s *SpyHealthRegistrar) Get(name string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[name]
}
