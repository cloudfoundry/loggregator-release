package v2_test

import (
	"doppler/internal/grpcmanager/v2"
	"io"
	"metricemitter/testhelper"
	plumbing "plumbing/v2"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ingress", func() {
	var (
		mockDataSetter  *mockDataSetter
		mockSender      *mockDopplerIngress_SenderServer
		mockBatchSender *mockBatcherSenderServer

		ingestor *v2.IngressServer
	)

	BeforeEach(func() {
		mockDataSetter = newMockDataSetter()
		mockSender = newMockDopplerIngress_SenderServer()
		mockBatchSender = newMockBatcherSenderServer()

		ingestor = v2.NewIngressServer(
			mockDataSetter,
			SpyBatcher{},
			testhelper.NewMetricClient(),
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
})

type SpyBatcher struct {
	metricbatcher.BatchCounterChainer
}

func (s SpyBatcher) BatchCounter(string) metricbatcher.BatchCounterChainer {
	return s
}

func (s SpyBatcher) Increment() {
}
