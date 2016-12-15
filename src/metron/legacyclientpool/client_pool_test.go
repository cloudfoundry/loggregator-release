//go:generate hel

package legacyclientpool_test

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"metron/legacyclientpool"
	"net"
	"os"
	"reflect"
	"testing"
	"testing/quick"
	"time"

	"github.com/a8m/expect"
	"github.com/apoydence/onpar"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Verbose() {
		log.SetOutput(ioutil.Discard)
	}

	os.Exit(m.Run())
}

type Expectation func(actual interface{}) *expect.Expect

func TestClientPool(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) (
		Expectation,
		*legacyclientpool.ClientPool,
		*net.UDPConn,
	) {
		expect := expect.New(t)
		addr, _ := net.ResolveUDPAddr("udp4", "localhost:0")
		conn, err := net.ListenUDP("udp4", addr)
		expect(err).To.Be.Nil()

		pool := legacyclientpool.New(conn.LocalAddr().String(), 10, 100*time.Millisecond)
		return expect, pool, conn
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
		cs, addrs := readFromUDP(conn)
		for i := 0; i < 5; i++ {
			pool.Write([]byte("some-data"))
		}

		expect(cs).To.Pass(eventuallyHaveLen{5})
		expect(addrs).To.Have.Len(5)
		m := toMap(addrs)
		expect(m).To.Have.Len(1)
	})

	o.Spec("refreshes the connection after maxWrites is hit", func(
		t *testing.T,
		expect Expectation,
		pool *legacyclientpool.ClientPool,
		conn *net.UDPConn,
	) {
		_, addrs := readFromUDP(conn)
		for i := 0; i < 15; i++ {
			pool.Write([]byte("some-data"))
		}

		expect(addrs).To.Pass(eventuallyHaveLen{15})
		m := toMap(addrs)
		expect(m).To.Have.Len(2)
	})

	o.Spec("refreshes the connection after refresh interval", func(
		t *testing.T,
		expect Expectation,
		pool *legacyclientpool.ClientPool,
		conn *net.UDPConn,
	) {
		_, addrs := readFromUDP(conn)

		pool.Write([]byte("some-data"))
		expect(addrs).To.Pass(eventuallyHaveLen{1})
		time.Sleep(101 * time.Millisecond)
		pool.Write([]byte("some-data"))
		expect(addrs).To.Pass(eventuallyHaveLen{2})
		m := toMap(addrs)
		expect(m).To.Have.Len(2)
	})
}

func TestInvalidAddr(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) (
		Expectation,
		*legacyclientpool.ClientPool,
	) {
		pool := legacyclientpool.New("!#$&!$#&", 10, 100*time.Millisecond)
		return expect.New(t), pool
	})

	o.Spec("it returns an error", func(
		t *testing.T,
		expect Expectation,
		pool *legacyclientpool.ClientPool,
	) {
		f := func() bool {
			return pool.Write([]byte("some-data")) != nil
		}
		err := quick.Check(f, nil)
		expect(err).To.Be.Nil()
	})
}

func toMap(c <-chan net.Addr) map[string]bool {
	m := make(map[string]bool)
	for {
		select {
		case x := <-c:
			m[x.String()] = true
		default:
			return m
		}
	}
}

func readFromUDP(conn *net.UDPConn) (<-chan []byte, <-chan net.Addr) {
	c := make(chan []byte, 100)
	addrs := make(chan net.Addr, 100)

	go func() {
		buffer := make([]byte, 1024)
		for {
			n, addr, err := conn.ReadFrom(buffer)
			if err != nil {
				return
			}

			c <- buffer[:n]
			addrs <- addr
		}
	}()

	return c, addrs
}

type receive struct{}

func (m receive) Match(actual interface{}) error {
	cs := actual.(<-chan []byte)
	select {
	case <-cs:
		return nil
	case <-time.After(1 * time.Second):
	}
	return fmt.Errorf("timed out waiting for data")
}

type eventuallyHaveLen struct {
	expected int
}

func (e eventuallyHaveLen) Match(actual interface{}) error {
	v := reflect.ValueOf(actual)
	for i := 0; i < 100; i++ {
		if v.Len() == e.expected {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}

	return fmt.Errorf("expected %v to have len %d", actual, e.expected)
}
