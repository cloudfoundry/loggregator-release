package clientpool_test

import (
	"errors"
	"metron/clientpool"
	"net"
	"plumbing"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/a8m/expect"
	"github.com/apoydence/onpar"
)

type Expectation func(actual interface{}) *expect.Expect

func TestConnManager(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) (Expectation, *clientpool.ConnManager, *mockFinder) {
		mockFinder := newMockFinder()
		return Expectation(expect.New(t)), clientpool.NewConnManager(5, mockFinder), mockFinder
	})

	o.Group("when no connection is present", func() {
		o.Spec("it returns an error", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
		) {
			err := conn.Write([]byte("some-data"))
			expect(err).Not.To.Be.Nil()
		})
	})

	o.Group("when a connection is present", func() {
		o.BeforeEach(func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
		) (*mockDopplerIngestorServer, func()) {
			grpcAddr, mockGRPCServer, grpcServerCleanup := startGRPCServer()
			finder.DopplerOutput.Uri <- grpcAddr

			return mockGRPCServer, grpcServerCleanup
		})

		o.AfterEach(func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
			mockServer *mockDopplerIngestorServer,
			grpcServerCleanup func(),
		) {
			//	grpcServerCleanup()
		})

		o.Spec("it sends the message down the connection", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
			mockServer *mockDopplerIngestorServer,
			_ func(),
		) {
			msg := []byte("some-data")
			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})

			rxMatcher := &fetchReceiver{tries: 100}
			expect(mockServer.PusherInput.Arg0).To.Pass(rxMatcher).Else.FailNow()

			value, err := rxMatcher.receiver.Recv()
			expect(err == nil).To.Equal(true)
			expect(value.Payload).To.Equal(msg)
		})

		o.Spec("it establishes a new connection on failure", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
			mockServer *mockDopplerIngestorServer,
			killGRPC func(),
		) {
			msg := []byte("some-data")
			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})
			killGRPC()

			grpcAddr, mockGRPCServer, grpcServerCleanup := startGRPCServer()
			finder.DopplerOutput.Uri <- grpcAddr
			defer grpcServerCleanup()

			// Write to it enough to surpass gRPC buffers
			for i := 0; i < 5; i++ {
				conn.Write(msg)
			}

			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})
			rxMatcher := &fetchReceiver{tries: 100}
			expect(mockGRPCServer.PusherInput.Arg0).To.Pass(rxMatcher).Else.FailNow()
		})

		o.Spec("it recycles connections after max writes", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
			mockServer *mockDopplerIngestorServer,
			killGRPC func(),
		) {
			msg := []byte("some-data")
			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})

			grpcAddr, mockGRPCServer, grpcServerCleanup := startGRPCServer()
			finder.DopplerOutput.Uri <- grpcAddr
			defer grpcServerCleanup()

			// Write until you surpass the maxWrite
			for i := 0; i < 6; i++ {
				conn.Write(msg)
			}

			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})
			rxMatcher := &fetchReceiver{tries: 100}
			expect(mockGRPCServer.PusherInput.Arg0).To.Pass(rxMatcher).Else.FailNow()
		})

	})
}

type eventuallyMsgWriter struct {
	msg   []byte
	tries int
}

func (e eventuallyMsgWriter) Match(actual interface{}) error {
	f := actual.(func([]byte) error)
	var err error
	for i := 0; i < e.tries; i++ {
		err = f(e.msg)
		if err == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return err
}

type eventuallyHasData struct {
	tries int
}

func (e eventuallyHasData) Match(actual interface{}) error {
	f := actual.(chan bool)
	var err error
	for i := 0; i < e.tries; i++ {
		if len(f) > 0 {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return err
}

type fetchReceiver struct {
	tries    int
	receiver plumbing.DopplerIngestor_PusherServer
}

func (r *fetchReceiver) Match(actual interface{}) error {
	c := actual.(chan plumbing.DopplerIngestor_PusherServer)

	for i := 0; i < r.tries; i++ {
		if len(c) > 0 {
			r.receiver = <-c
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("receive")
}

func startGRPCServer() (string, *mockDopplerIngestorServer, func()) {
	mockDoppler := newMockDopplerIngestorServer()

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	plumbing.RegisterDopplerIngestorServer(s, mockDoppler)

	go s.Serve(lis)

	println("ADDR", lis.Addr().String(), mockDoppler.PusherInput.Arg0)
	return lis.Addr().String(), mockDoppler, func() {
		s.Stop()
		lis.Close()
	}
}
