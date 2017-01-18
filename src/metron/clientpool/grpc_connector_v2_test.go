package clientpool_test

import (
	"errors"
	"plumbing/v2"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"metron/clientpool"
)

var _ = Describe("GRPCV2Connector", func() {
	Context("when successfully connecting to the AZ", func() {
		var (
			df               *mockDialFunc
			cf               *mockIngressClientFunc
			mockSender       *mockDopplerIngressClient
			mockSenderClient *mockDopplerIngress_SenderClient
			clientConn       *grpc.ClientConn
		)

		BeforeEach(func() {
			df = newMockDialFunc()
			clientConn = &grpc.ClientConn{}
			df.retClientConn <- clientConn
			df.retErr <- nil

			cf = newMockIngressClientFunc()
			mockSender = newMockDopplerIngressClient()
			mockSenderClient = newMockDopplerIngress_SenderClient()

			cf.retIngressClient <- mockSender
			mockSender.SenderOutput.Ret0 <- mockSenderClient
			mockSender.SenderOutput.Ret1 <- nil
		})

		It("connects to the dns name with az prefix", func() {
			connector := clientpool.MakeV2Connector("test-name", "z1", df.fn, cf.fn, grpc.WithInsecure())
			_, _, err := connector.Connect()
			Expect(err).ToNot(HaveOccurred())

			Expect(df.inputDoppler).To(Receive(Equal("z1.test-name")))
		})

		It("returns the original client connection", func() {
			connector := clientpool.MakeV2Connector("test-name", "", df.fn, cf.fn, grpc.WithInsecure())
			conn, _, err := connector.Connect()
			Expect(err).ToNot(HaveOccurred())

			Expect(conn).To(Equal(clientConn))
		})

		It("returns the pusher client", func() {
			connector := clientpool.MakeV2Connector("test-name", "", df.fn, cf.fn, grpc.WithInsecure())
			_, pusherClient, err := connector.Connect()
			Expect(err).ToNot(HaveOccurred())

			Expect(pusherClient).To(Equal(mockSenderClient))
		})
	})

	Context("when unable to connect to AZ specific dopplers", func() {
		It("dials the original dns name", func() {
			df := newMockDialFunc()
			cf := newMockIngressClientFunc()
			mockSender := newMockDopplerIngressClient()
			mockSenderClient := newMockDopplerIngress_SenderClient()

			df.retClientConn <- newMockClientConn()
			df.retErr <- nil
			mockSender.SenderOutput.Ret0 <- nil
			mockSender.SenderOutput.Ret1 <- errors.New("fake error")
			cf.retIngressClient <- mockSender

			df.retClientConn <- &grpc.ClientConn{}
			df.retErr <- nil
			mockSender.SenderOutput.Ret0 <- mockSenderClient
			mockSender.SenderOutput.Ret1 <- nil
			cf.retIngressClient <- mockSender

			connector := clientpool.MakeV2Connector("test-name", "z1", df.fn, cf.fn)
			_, _, err := connector.Connect()
			Expect(err).ToNot(HaveOccurred())

			Expect(df.inputDoppler).To(Receive(Equal("z1.test-name")))
			Expect(df.inputDoppler).To(Receive(Equal("test-name")))
		})
	})

	Context("when unable to connect to any doppler", func() {
		It("returns an error", func() {
			df := newMockDialFunc()

			df.retClientConn <- nil
			df.retErr <- errors.New("fake error")

			df.retClientConn <- nil
			df.retErr <- errors.New("fake error")

			connector := clientpool.MakeV2Connector("test-name", "z1", df.fn, nil)
			_, _, err := connector.Connect()
			Expect(err).To(HaveOccurred())
		})
	})
})

type mockIngressClientFunc struct {
	inputClientConn  chan *grpc.ClientConn
	retIngressClient chan loggregator.DopplerIngressClient
	fn               clientpool.SenderClientFunc
}

func newMockIngressClientFunc() *mockIngressClientFunc {
	cf := &mockIngressClientFunc{
		inputClientConn:  make(chan *grpc.ClientConn, 100),
		retIngressClient: make(chan loggregator.DopplerIngressClient, 100),
	}
	cf.fn = func(conn *grpc.ClientConn) loggregator.DopplerIngressClient {
		cf.inputClientConn <- conn
		return <-cf.retIngressClient
	}
	return cf
}
