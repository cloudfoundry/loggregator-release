//go:generate hel

package legacyclientpool_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"metron/legacyclientpool"
	"net"
	"testing"
	"time"

	"github.com/a8m/expect"
	"github.com/apoydence/onpar"
)

type Expectation func(actual interface{}) *expect.Expect

func TestClientPool(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)
	log.SetOutput(ioutil.Discard)

	o.BeforeEach(func(t *testing.T) (
		Expectation,
		*legacyclientpool.ClientPool,
		*net.UDPConn,
	) {
		expect := expect.New(t)
		addr, _ := net.ResolveUDPAddr("udp4", "localhost:0")
		conn, err := net.ListenUDP("udp4", addr)
		expect(err).To.Be.Nil()

		return expect, legacyclientpool.New(conn.LocalAddr().String(), 10), conn
	})

	o.AfterEach(func(
		t *testing.T,
		expect Expectation,
		pool *legacyclientpool.ClientPool,
		conn *net.UDPConn,
	) {
		conn.Close()
	})

	o.Spec("writes messages to doppler", func(
		t *testing.T,
		expect Expectation,
		pool *legacyclientpool.ClientPool,
		conn *net.UDPConn,
	) {
		cs := readFromUDP(conn)
		go func() {
			for i := 0; i < 100; i++ {
				pool.Write([]byte("some-data"))
				time.Sleep(10 * time.Millisecond)
			}
		}()

		expect(cs).To.Pass(receive{})
	})
}

func readFromUDP(conn *net.UDPConn) <-chan []byte {
	c := make(chan []byte)
	go func() {
		buffer := make([]byte, 1024)
		for {
			n, err := conn.Read(buffer)
			if err != nil {
				return
			}

			c <- buffer[:n]
		}
	}()

	return c
}

type receive struct {
}

func (m receive) Match(actual interface{}) error {
	cs := actual.(<-chan []byte)
	select {
	case <-cs:
		return nil
	case <-time.After(1 * time.Second):
	}
	return fmt.Errorf("timed out waiting for data")
}
