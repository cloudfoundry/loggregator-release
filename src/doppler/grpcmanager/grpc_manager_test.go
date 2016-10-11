package grpcmanager_test

import (
	"doppler/grpcmanager"
	"io"
	"net"
	"plumbing"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServerHandler", func() {
	var (
		mockRegistrar *mockRegistrar
		mockCleanup   func()
		cleanupCalled chan struct{}

		manager       *grpcmanager.GrpcManager
		listener      net.Listener
		connCloser    io.Closer
		dopplerClient plumbing.DopplerClient

		streamRequest   *plumbing.StreamRequest
		firehoseRequest *plumbing.FirehoseRequest

		setter grpcmanager.DataSetter
	)

	var startGrpcServer = func(ds plumbing.DopplerServer) net.Listener {
		lis, err := net.Listen("tcp", ":0")
		Expect(err).ToNot(HaveOccurred())
		s := grpc.NewServer()
		plumbing.RegisterDopplerServer(s, ds)
		go s.Serve(lis)

		return lis
	}

	var establishClient = func(dopplerAddr string) (plumbing.DopplerClient, io.Closer) {
		conn, err := grpc.Dial(dopplerAddr, grpc.WithInsecure())
		Expect(err).ToNot(HaveOccurred())
		c := plumbing.NewDopplerClient(conn)

		return c, conn
	}

	var fetchSetter = func() grpcmanager.DataSetter {
		var s grpcmanager.DataSetter
		Eventually(mockRegistrar.RegisterInput.Setter).Should(
			Receive(&s),
		)
		return s
	}

	var buildCleanup = func(c chan struct{}) func() {
		return func() {
			close(c)
		}
	}

	BeforeEach(func() {
		mockRegistrar = newMockRegistrar()
		cleanupCalled = make(chan struct{})
		mockCleanup = buildCleanup(cleanupCalled)
		mockRegistrar.RegisterOutput.Ret0 <- mockCleanup

		manager = grpcmanager.New(mockRegistrar)

		listener = startGrpcServer(manager)
		dopplerClient, connCloser = establishClient(listener.Addr().String())

		streamRequest = &plumbing.StreamRequest{AppID: "some-app-id"}
		firehoseRequest = &plumbing.FirehoseRequest{SubID: "some-sub-id"}
	})

	AfterEach(func() {
		connCloser.Close()
		listener.Close()
	})

	Describe("registration", func() {
		It("registers stream", func() {
			_, err := dopplerClient.Stream(context.TODO(), streamRequest)
			Expect(err).ToNot(HaveOccurred())

			Eventually(mockRegistrar.RegisterInput).Should(
				BeCalled(With(streamRequest.AppID, false, Not(BeNil()))),
			)
		})

		It("registers firehose", func() {
			_, err := dopplerClient.Firehose(context.TODO(), firehoseRequest)
			Expect(err).ToNot(HaveOccurred())

			Eventually(mockRegistrar.RegisterInput).Should(
				BeCalled(With(firehoseRequest.SubID, true, Not(BeNil()))),
			)
		})

		Context("connection is established", func() {
			Context("client does not close the connection", func() {
				It("does not unregister itself", func() {
					dopplerClient.Stream(context.TODO(), streamRequest)
					fetchSetter()

					Consistently(cleanupCalled).ShouldNot(BeClosed())
				})
			})

			Context("client closes connection", func() {
				It("unregisters stream", func() {
					dopplerClient.Stream(context.TODO(), streamRequest)
					setter = fetchSetter()
					connCloser.Close()

					Eventually(cleanupCalled).Should(BeClosed())
				})

				It("unregisters firehose", func() {
					dopplerClient.Firehose(context.TODO(), firehoseRequest)
					setter = fetchSetter()
					connCloser.Close()

					Eventually(cleanupCalled).Should(BeClosed())
				})

			})
		})
	})

	Describe("data transmission", func() {
		var readFromReceiver = func(r plumbing.Doppler_StreamClient) <-chan []byte {
			c := make(chan []byte, 100)

			go func() {
				for {
					resp, err := r.Recv()
					if err != nil {
						return
					}

					c <- resp.Payload
				}
			}()

			return c
		}

		It("stream sends data from the setter to the client", func() {
			rx, err := dopplerClient.Stream(context.TODO(), streamRequest)
			Expect(err).ToNot(HaveOccurred())

			setter = fetchSetter()
			setter.Set([]byte("some-data-0"))
			setter.Set([]byte("some-data-1"))
			setter.Set([]byte("some-data-2"))

			c := readFromReceiver(rx)
			Eventually(c).Should(BeCalled(With(
				[]byte("some-data-0"),
				[]byte("some-data-1"),
				[]byte("some-data-2"),
			)))
		})

		It("firehose sends data from the setter to the client", func() {
			rx, err := dopplerClient.Firehose(context.TODO(), firehoseRequest)
			Expect(err).ToNot(HaveOccurred())

			setter = fetchSetter()
			setter.Set([]byte("some-data-0"))
			setter.Set([]byte("some-data-1"))
			setter.Set([]byte("some-data-2"))

			c := readFromReceiver(rx)
			Eventually(c).Should(BeCalled(With(
				[]byte("some-data-0"),
				[]byte("some-data-1"),
				[]byte("some-data-2"),
			)))
		})
	})

	Describe("metrics", func() {
		//TODO
	})
})
