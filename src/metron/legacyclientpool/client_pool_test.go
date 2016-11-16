//go:generate hel

package legacyclientpool_test

import (
	"doppler/dopplerservice"
	"fmt"
	"io/ioutil"
	"log"
	"metron/legacyclientpool"
	"net"
	"reflect"
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
		*mockFinder,
	) {
		finder := newMockFinder()
		return expect.New(t), legacyclientpool.New(finder), finder
	})

	o.Group("when dopplers are available", func() {
		o.BeforeEach(func(
			t *testing.T,
			expect Expectation,
			pool *legacyclientpool.ClientPool,
			finder *mockFinder,
		) []*net.UDPConn {
			var conns []*net.UDPConn
			var addrs []string
			for i := 0; i < 5; i++ {
				addr, _ := net.ResolveUDPAddr("udp4", ":0")
				conn, err := net.ListenUDP("udp4", addr)
				expect(err).To.Be.Nil()
				conns = append(conns, conn)

				addrs = append(addrs, conn.LocalAddr().String())
			}

			finder.NextOutput.Ret0 <- dopplerservice.Event{
				UDPDopplers: addrs,
			}

			return conns
		})

		o.AfterEach(func(
			t *testing.T,
			expect Expectation,
			pool *legacyclientpool.ClientPool,
			finder *mockFinder,
			conns []*net.UDPConn,
		) {
			for _, conn := range conns {
				conn.Close()
			}
		})

		o.Spec("it writes to a random doppler", func(
			t *testing.T,
			expect Expectation,
			pool *legacyclientpool.ClientPool,
			finder *mockFinder,
			conns []*net.UDPConn,
		) {
			var cs []<-chan []byte
			for _, c := range conns {
				cs = append(cs, readFromUDP(c))
			}

			go func() {
				for i := 0; i < 100; i++ {
					pool.Write([]byte("some-data"))
					time.Sleep(10 * time.Millisecond)
				}
			}()

			expect(cs).To.Pass(readFromAll{})
		})
	})

	o.Group("when no dopplers are available", func() {
		o.BeforeEach(func(
			t *testing.T,
			expect Expectation,
			pool *legacyclientpool.ClientPool,
			finder *mockFinder,
		) {
			close(finder.NextOutput.Ret0)
		})

		o.Spec("it returns an error", func(
			t *testing.T,
			expect Expectation,
			pool *legacyclientpool.ClientPool,
			finder *mockFinder,
		) {
			err := pool.Write([]byte("some-data"))
			expect(err).Not.To.Be.Nil()
		})
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

type readFromAll struct {
}

func (m readFromAll) Match(actual interface{}) error {
	cs := actual.([]<-chan []byte)
	for i := 0; i < 100; i++ {
		var cases []reflect.SelectCase
		for _, c := range cs {
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(c),
			})
		}

		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(time.After(5 * time.Second)),
		})

		caseIdx, v, _ := reflect.Select(cases)

		if caseIdx == len(cases)-1 {
			return fmt.Errorf("timed out waiting for data")
		}

		_ = v
		cs = append(cs[:caseIdx], cs[caseIdx+1:]...)
		if len(cs) == 0 {
			return nil
		}
	}

	return fmt.Errorf("to read from all the connections")
}
