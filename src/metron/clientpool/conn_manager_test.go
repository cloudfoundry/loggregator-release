package clientpool_test

import (
	"errors"
	"fmt"
	"integration_tests"
	"io/ioutil"
	"log"
	"metron/clientpool"
	"metron/testutil"
	"os"
	"plumbing"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"

	"github.com/a8m/expect"
	"github.com/apoydence/onpar"
)

func TestMain(m *testing.M) {
	if !testing.Verbose() {
		log.SetOutput(ioutil.Discard)
		grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))
	}

	os.Exit(m.Run())
}

type Expectation func(actual interface{}) *expect.Expect

func TestConnManager(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) (Expectation, *clientpool.ConnManager, *testutil.Server) {
		expect := Expectation(expect.New(t))

		tlsConfig, err := plumbing.NewMutualTLSConfig(
			integration_tests.ClientCertFilePath(),
			integration_tests.ClientKeyFilePath(),
			integration_tests.CAFilePath(),
			"doppler",
		)
		expect(err).To.Be.Nil().Else.FailNow()

		grpcServer, err := testutil.NewServer()
		expect(err).To.Be.Nil().Else.FailNow()

		dialer := clientpool.NewDialer(
			grpcServer.URI(),
			"z1",
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		)
		return expect, clientpool.NewConnManager(dialer, 5), grpcServer
	})

	o.Group("when a connection is present", func() {
		o.Spec("it sends the message down the connection", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
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

		o.Spec("it recycles connections after max writes", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			grpcServer *testutil.Server,
		) {
			msg := []byte("some-data")
			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})
			// Write until you surpass the maxWrite
			for i := 0; i < 6; i++ {
				conn.Write(msg)
			}

			expect(conn.Write).To.Pass(eventuallyMsgWriter{tries: 100, msg: msg})
			haveLen := &eventuallyHasLen{tries: 100, desired: 2}
			expect(grpcServer.PusherCalled).To.Pass(haveLen).Else.FailNow()
		})
	})
}

func TestConnManagerBadDNS(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) (Expectation, *clientpool.ConnManager) {
		expect := Expectation(expect.New(t))

		tlsConfig, err := plumbing.NewMutualTLSConfig(
			integration_tests.ClientCertFilePath(),
			integration_tests.ClientKeyFilePath(),
			integration_tests.CAFilePath(),
			"doppler",
		)
		expect(err).To.Be.Nil().Else.FailNow()

		dialer := clientpool.NewDialer(
			"invalid",
			"z1",
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		)
		return expect, clientpool.NewConnManager(dialer, 5)
	})

	o.Spec("write returns an error", func(
		t *testing.T,
		expect Expectation,
		conn *clientpool.ConnManager,
	) {
		err := conn.Write([]byte("some-data"))
		expect(err == nil).To.Equal(false)
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

type eventuallyHasLen struct {
	tries   int
	desired int
}

func (e eventuallyHasLen) Match(actual interface{}) error {
	f := actual.(chan bool)
	for i := 0; i < e.tries; i++ {
		if len(f) == e.desired {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("expected to have len %d", e.desired)
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
