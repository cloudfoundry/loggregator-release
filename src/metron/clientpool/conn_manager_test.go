package clientpool_test

import (
	"metron/clientpool"
	"net"
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
		) *net.UDPConn {
			ip, err := net.ResolveUDPAddr("udp4", "localhost:0")
			expect(err).To.Be.Nil().Else.FailNow()
			udpConn, err := net.ListenUDP("udp4", ip)
			expect(err).To.Be.Nil().Else.FailNow()
			finder.DopplerOutput.Uri <- udpConn.LocalAddr().String()

			return udpConn
		})

		o.Spec("it sends the message down the connection", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
			udpConn *net.UDPConn,
		) {
			msg := []byte("some-data")
			expect(conn.Write).To.Pass(eventualMsgWriter{msg: msg, tries: 10})

			udpConn.SetReadDeadline(time.Now().Add(time.Second))
			buffer := make([]byte, 1024)
			n, err := udpConn.Read(buffer)
			expect(err).To.Be.Nil().Else.FailNow()
			expect(buffer[:n]).To.Equal(msg)
		})
	})

	o.Group("when multiple dopplers are available", func() {
		o.BeforeEach(func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
		) (*net.UDPConn, *net.UDPConn) {
			ip, err := net.ResolveUDPAddr("udp4", "localhost:0")
			expect(err).To.Be.Nil().Else.FailNow()

			udpConn1, err := net.ListenUDP("udp4", ip)
			expect(err).To.Be.Nil().Else.FailNow()
			finder.DopplerOutput.Uri <- udpConn1.LocalAddr().String()

			udpConn2, err := net.ListenUDP("udp4", ip)
			expect(err).To.Be.Nil().Else.FailNow()
			finder.DopplerOutput.Uri <- udpConn2.LocalAddr().String()

			return udpConn1, udpConn2
		})

		o.Spec("it resets the connection after a given number of writes", func(
			t *testing.T,
			expect Expectation,
			conn *clientpool.ConnManager,
			finder *mockFinder,
			udpConn1, udpConn2 *net.UDPConn,
		) {
			msg := []byte("some-data")
			for i := 0; i < 5; i++ {
				expect(conn.Write).To.Pass(eventualMsgWriter{msg: msg, tries: 10})
				udpConn1.SetReadDeadline(time.Now().Add(time.Second))
				buffer := make([]byte, 1024)
				n, err := udpConn1.Read(buffer)
				expect(err).To.Be.Nil().Else.FailNow()
				expect(buffer[:n]).To.Equal(msg)
			}

			expect(conn.Write).To.Pass(eventualMsgWriter{msg: msg, tries: 10})
			udpConn2.SetReadDeadline(time.Now().Add(time.Second))
			buffer := make([]byte, 1024)
			n, err := udpConn2.Read(buffer)
			expect(err).To.Be.Nil().Else.FailNow()
			expect(buffer[:n]).To.Equal(msg)
		})
	})
}

type eventualMsgWriter struct {
	msg   []byte
	tries int
}

func (e eventualMsgWriter) Match(actual interface{}) error {
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
