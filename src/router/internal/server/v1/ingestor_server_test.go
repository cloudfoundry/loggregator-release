package v1_test

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"code.cloudfoundry.org/go-loggregator/v9/conversion"
	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"
	v1 "code.cloudfoundry.org/loggregator/router/internal/server/v1"
	"github.com/cloudfoundry/sonde-go/events"
	"google.golang.org/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IngestorServer", func() {
	var startGRPCServer = func(ds plumbing.DopplerIngestorServer) (*grpc.Server, string) {
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).ToNot(HaveOccurred())
		s := grpc.NewServer()
		plumbing.RegisterDopplerIngestorServer(s, ds)
		go func() {
			_ = s.Serve(lis)
		}()

		return s, lis.Addr().String()
	}

	var establishClient = func(dopplerAddr string) (plumbing.DopplerIngestorClient, io.Closer) {
		conn, err := grpc.Dial(dopplerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		Expect(err).ToNot(HaveOccurred())
		c := plumbing.NewDopplerIngestorClient(conn)

		return c, conn
	}

	var (
		v1Buf         *diodes.ManyToOneEnvelope
		v2Buf         *diodes.ManyToOneEnvelopeV2
		manager       *v1.IngestorServer
		server        *grpc.Server
		connCloser    io.Closer
		dopplerClient plumbing.DopplerIngestorClient
		ingressMetric *metricemitter.Counter
	)

	BeforeEach(func() {
		var grpcAddr string
		v1Buf = diodes.NewManyToOneEnvelope(5, nil)
		v2Buf = diodes.NewManyToOneEnvelopeV2(5, nil)
		ingressMetric = metricemitter.NewCounter("ingress", "doppler")

		manager = v1.NewIngestorServer(
			v1Buf,
			v2Buf,
			ingressMetric,
		)
		server, grpcAddr = startGRPCServer(manager)
		dopplerClient, connCloser = establishClient(grpcAddr)
	})

	AfterEach(func() {
		server.Stop()
		connCloser.Close()
	})

	It("reads envelopes from ingestor client into the v1 Buffer", func() {
		pusherClient, err := dopplerClient.Pusher(context.TODO())
		Expect(err).ToNot(HaveOccurred())

		someEnvelope, data := buildContainerMetric()
		err = pusherClient.Send(&plumbing.EnvelopeData{Payload: data})
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			env, ok := v1Buf.TryNext()
			if !ok {
				return false
			}
			return proto.Equal(env, someEnvelope)
		}).Should(BeTrue())
		Expect(ingressMetric.GetDelta()).To(BeNumerically(">", 0))
	})

	It("reads envelopes from ingestor client into the v2 Buffer", func() {
		pusherClient, err := dopplerClient.Pusher(context.TODO())
		Expect(err).ToNot(HaveOccurred())

		someEnvelope, data := buildContainerMetric()
		err = pusherClient.Send(&plumbing.EnvelopeData{Payload: data})
		Expect(err).ToNot(HaveOccurred())

		v2e := conversion.ToV2(someEnvelope, true)
		Eventually(v2Buf.Next).Should(Equal(v2e))
	})

	Context("With an unsupported envelope payload", func() {
		It("does not forward the message to the sender", func() {
			pusherClient, err := dopplerClient.Pusher(context.TODO())
			Expect(err).ToNot(HaveOccurred())

			err = pusherClient.Send(&plumbing.EnvelopeData{Payload: []byte("unsupported envelope")})
			Expect(err).ToNot(HaveOccurred())
			Consistently(func() bool {
				_, ok := v1Buf.TryNext()
				return ok
			}).Should(BeFalse())

			err = pusherClient.Send(&plumbing.EnvelopeData{})
			Expect(err).ToNot(HaveOccurred())
			Consistently(func() bool {
				_, ok := v1Buf.TryNext()
				return ok
			}).Should(BeFalse())
		})
	})

	Context("When the Recv returns an EOF error", func() {
		It("exits the function gracefully", func() {
			fakeStream := newSpyIngestorGRPCServer()
			fakeStream.recvError = io.EOF

			Eventually(func() error {
				return manager.Pusher(fakeStream)
			}).Should(Succeed())
			Consistently(func() bool {
				_, ok := v1Buf.TryNext()
				return ok
			}).Should(BeFalse())
		})
	})

	Context("When the Recv returns an error", func() {
		It("does not forward the message to the sender", func() {
			ctx, cancel := context.WithCancel(context.Background())
			fakeStream := newSpyIngestorGRPCServer()
			fakeStream.context = ctx
			fakeStream.recvError = errors.New("fake error")

			errCh := make(chan error)
			go func() {
				errCh <- manager.Pusher(fakeStream)
			}()

			Consistently(func() bool {
				_, ok := v1Buf.TryNext()
				return ok
			}).Should(BeFalse())

			cancel()
			Expect(<-errCh).To(MatchError("context canceled"))
		})
	})

	Context("When the pusher context finishes", func() {
		It("returns the error from the context", func() {
			fakeStream := newSpyIngestorGRPCServer()

			for i := 0; i < 100; i++ {
				fakeStream.recvError = errors.New("fake error")
			}

			context, cancelCtx := context.WithCancel(context.Background())
			fakeStream.context = context
			cancelCtx()

			err := manager.Pusher(fakeStream)

			Expect(err).To(HaveOccurred())
		})
	})
})

func buildContainerMetric() (*events.Envelope, []byte) {
	envelope := &events.Envelope{
		Origin:    proto.String("doppler"),
		EventType: events.Envelope_ContainerMetric.Enum(),
		Timestamp: proto.Int64(time.Now().UnixNano()),
		ContainerMetric: &events.ContainerMetric{
			ApplicationId: proto.String("some-app"),
			InstanceIndex: proto.Int32(int32(1)),
			CpuPercentage: proto.Float64(float64(1)),
			MemoryBytes:   proto.Uint64(uint64(1)),
			DiskBytes:     proto.Uint64(uint64(1)),
		},
	}
	data, err := proto.Marshal(envelope)
	Expect(err).ToNot(HaveOccurred())
	return envelope, data
}

func newSpyIngestorGRPCServer() *spyIngestorGRPCServer {
	return &spyIngestorGRPCServer{}
}

type spyIngestorGRPCServer struct {
	context          context.Context
	recvEnvelopeData *plumbing.EnvelopeData
	recvError        error
	grpc.ServerStream
}

func (s *spyIngestorGRPCServer) Context() context.Context {
	if s.context == nil {
		return context.TODO()
	}
	return s.context
}

func (s *spyIngestorGRPCServer) SendAndClose(r *plumbing.PushResponse) error {
	return nil
}

func (s *spyIngestorGRPCServer) Recv() (*plumbing.EnvelopeData, error) {
	return s.recvEnvelopeData, s.recvError
}
