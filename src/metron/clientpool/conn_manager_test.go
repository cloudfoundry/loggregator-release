package clientpool_test

import (
	"errors"
	"integration_tests"
	"metron/clientpool"
	"metron/testutil"
	"plumbing"
	"testing"
	"time"

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
		tlsConfig, err := plumbing.NewMutualTLSConfig(
			integration_tests.ClientCertFilePath(),
			integration_tests.ClientKeyFilePath(),
			integration_tests.CAFilePath(),
			"doppler",
		)
		expect := Expectation(expect.New(t))
		expect(err).To.Be.Nil().Else.FailNow()
		return expect, clientpool.NewConnManager(tlsConfig, 5, mockFinder), mockFinder
	})

	o.Group("when no connection is present", func() {
		o.Spec("it returns an error", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
		) {
			err := conn.Write([]byte("some-data"))
			expect(err).Not.To.Be.Nil().Else.FailNow()
		})
	})

	o.Group("when a connection is present", func() {
		o.BeforeEach(func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
		) *testutil.Server {
			grpcServer, err := testutil.NewServer()
			expect(err).To.Be.Nil().Else.FailNow()
			finder.DopplerOutput.Uri <- grpcServer.URI()
			return grpcServer
		})

		o.Spec("it sends the message down the connection", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
			grpcServer *testutil.Server,
		) {
			msg := []byte("some-data")
			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})

			rxMatcher := &fetchReceiver{tries: 100}
			expect(grpcServer.PusherInput.Arg0).To.Pass(rxMatcher).Else.FailNow()

			value, err := rxMatcher.receiver.Recv()
			expect(err == nil).To.Equal(true)
			expect(value.Payload).To.Equal(msg)

			err = grpcServer.Stop()
			expect(err).To.Be.Nil().Else.FailNow()
		})

		o.Spec("it establishes a new connection on failure", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
			grpcServer *testutil.Server,
		) {
			msg := []byte("some-data")
			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})

			err := grpcServer.Stop()
			expect(err).To.Be.Nil().Else.FailNow()

			grpcServer, err = testutil.NewServer()
			expect(err).To.Be.Nil().Else.FailNow()
			finder.DopplerOutput.Uri <- grpcServer.URI()

			// Write to it enough to surpass gRPC buffers
			for i := 0; i < 5; i++ {
				conn.Write(msg)
			}

			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})
			rxMatcher := &fetchReceiver{tries: 100}
			expect(grpcServer.PusherInput.Arg0).To.Pass(rxMatcher).Else.FailNow()

			err = grpcServer.Stop()
			expect(err).To.Be.Nil().Else.FailNow()
		})

		o.Spec("it recycles connections after max writes", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
			grpcServer *testutil.Server,
		) {
			msg := []byte("some-data")
			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})

			grpcServer, err := testutil.NewServer()
			expect(err).To.Be.Nil().Else.FailNow()
			finder.DopplerOutput.Uri <- grpcServer.URI()

			// Write until you surpass the maxWrite
			for i := 0; i < 6; i++ {
				conn.Write(msg)
			}

			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})
			rxMatcher := &fetchReceiver{tries: 100}
			expect(grpcServer.PusherInput.Arg0).To.Pass(rxMatcher).Else.FailNow()

			err = grpcServer.Stop()
			expect(err).To.Be.Nil().Else.FailNow()
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
