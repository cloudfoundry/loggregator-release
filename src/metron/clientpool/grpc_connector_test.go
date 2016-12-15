package clientpool_test

import (
	"errors"
	"plumbing"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"metron/clientpool"
)

var _ = Describe("GRPCConnector", func() {
	Context("when successfully connecting to the AZ", func() {
		var (
			// todo rename with prefix mock
			df               *mockDialFunc
			cf               *mockIngestorClientFunc
			mockPusher       *mockPusher
			mockPusherClient *mockDopplerIngestor_PusherClient
			clientConn       *grpc.ClientConn
		)

		BeforeEach(func() {
			df = newMockDialFunc()
			clientConn = &grpc.ClientConn{}
			df.retClientConn <- clientConn
			df.retErr <- nil

			cf = newMockIngestorClientFunc()
			mockPusher = newMockPusher()
			mockPusherClient = newMockDopplerIngestor_PusherClient()

			cf.retIngestorClient <- mockPusher
			mockPusher.PusherOutput.Ret0 <- mockPusherClient
			mockPusher.PusherOutput.Ret1 <- nil
		})

		It("connects to the dns name with az prefix", func() {
			connector := clientpool.MakeGRPCConnector("test-name", "z1", df.fn, cf.fn, grpc.WithInsecure())
			_, _, err := connector.Connect()
			Expect(err).ToNot(HaveOccurred())

			Expect(df.inputDoppler).To(Receive(Equal("z1.test-name")))
		})

		It("returns the original client connection", func() {
			connector := clientpool.MakeGRPCConnector("test-name", "", df.fn, cf.fn, grpc.WithInsecure())
			conn, _, err := connector.Connect()
			Expect(err).ToNot(HaveOccurred())

			Expect(conn).To(Equal(clientConn))
		})

		It("returns the pusher client", func() {
			connector := clientpool.MakeGRPCConnector("test-name", "", df.fn, cf.fn, grpc.WithInsecure())
			_, pusherClient, err := connector.Connect()
			Expect(err).ToNot(HaveOccurred())

			Expect(pusherClient).To(Equal(mockPusherClient))
		})
	})

	Context("when unable to connect to AZ specific dopplers", func() {
		It("dials the original dns name", func() {
			df := newMockDialFunc()
			cf := newMockIngestorClientFunc()
			mockPusher := newMockPusher()
			mockPusherClient := newMockDopplerIngestor_PusherClient()

			df.retClientConn <- newMockClientConn()
			df.retErr <- nil
			mockPusher.PusherOutput.Ret0 <- nil
			mockPusher.PusherOutput.Ret1 <- errors.New("fake error")
			cf.retIngestorClient <- mockPusher

			df.retClientConn <- &grpc.ClientConn{}
			df.retErr <- nil
			mockPusher.PusherOutput.Ret0 <- mockPusherClient
			mockPusher.PusherOutput.Ret1 <- nil
			cf.retIngestorClient <- mockPusher

			connector := clientpool.MakeGRPCConnector("test-name", "z1", df.fn, cf.fn)
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

			connector := clientpool.MakeGRPCConnector("test-name", "z1", df.fn, nil)
			_, _, err := connector.Connect()
			Expect(err).To(HaveOccurred())
		})
	})
})

type mockDialFunc struct {
	inputDoppler     chan string
	inputDialOptions chan []grpc.DialOption
	retClientConn    chan *grpc.ClientConn
	retErr           chan error
	fn               clientpool.DialFunc
}

func newMockDialFunc() *mockDialFunc {
	df := &mockDialFunc{
		inputDoppler:     make(chan string, 100),
		inputDialOptions: make(chan []grpc.DialOption, 100),
		retClientConn:    make(chan *grpc.ClientConn, 100),
		retErr:           make(chan error, 100),
	}
	df.fn = func(doppler string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		df.inputDoppler <- doppler
		df.inputDialOptions <- opts
		return <-df.retClientConn, <-df.retErr
	}
	return df
}

type mockIngestorClientFunc struct {
	inputClientConn   chan *grpc.ClientConn
	retIngestorClient chan plumbing.DopplerIngestorClient
	fn                clientpool.IngestorClientFunc
}

func newMockIngestorClientFunc() *mockIngestorClientFunc {
	cf := &mockIngestorClientFunc{
		inputClientConn:   make(chan *grpc.ClientConn, 100),
		retIngestorClient: make(chan plumbing.DopplerIngestorClient, 100),
	}
	cf.fn = func(conn *grpc.ClientConn) plumbing.DopplerIngestorClient {
		cf.inputClientConn <- conn
		return <-cf.retIngestorClient
	}
	return cf
}

func newMockClientConn() *grpc.ClientConn {
	conn, err := grpc.Dial("", grpc.WithInsecure())
	Expect(err).NotTo(HaveOccurred())
	return conn
}
