//go:generate hel

package legacyclientpool_test

import (
	"metron/legacyclientpool"
	"net"
	"testing/quick"
	"time"

	"github.com/a8m/expect"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type Expectation func(actual interface{}) *expect.Expect

var _ = Describe("ClientPool", func() {
	var (
		pool *legacyclientpool.ClientPool
		conn *net.UDPConn
	)

	BeforeEach(func() {
		var err error
		addr, _ := net.ResolveUDPAddr("udp4", "localhost:0")
		conn, err = net.ListenUDP("udp4", addr)
		Expect(err).ToNot(HaveOccurred())

		pool = legacyclientpool.New(conn.LocalAddr().String(), 10, 100*time.Millisecond)
	})

	AfterEach(func() {
		conn.Close()
	})

	It("writes messages to doppler", func() {
		cs, addrs := readFromUDP(conn)
		for i := 0; i < 5; i++ {
			pool.Write([]byte("some-data"))
		}

		Eventually(cs).Should(HaveLen(5))
		Expect(addrs).To(HaveLen(5))
		m := toMap(addrs)
		Expect(m).To(HaveLen(1))
	})

	It("refreshes the connection after maxWrites is hit", func() {
		_, addrs := readFromUDP(conn)
		for i := 0; i < 15; i++ {
			pool.Write([]byte("some-data"))
		}

		Eventually(addrs).Should(HaveLen(15))
		m := toMap(addrs)
		Expect(m).To(HaveLen(2))
	})

	It("refreshes the connection after refresh interval", func() {
		_, addrs := readFromUDP(conn)

		pool.Write([]byte("some-data"))
		Eventually(addrs).Should(HaveLen(1))
		time.Sleep(101 * time.Millisecond)
		pool.Write([]byte("some-data"))
		Eventually(addrs).Should(HaveLen(2))
		m := toMap(addrs)
		Expect(m).To(HaveLen(2))
	})
})

var _ = Describe("Invalid Addr", func() {
	var pool *legacyclientpool.ClientPool
	BeforeEach(func() {
		pool = legacyclientpool.New("!#$&!$#&", 10, 100*time.Millisecond)
	})

	It("it returns an error", func() {
		f := func() bool {
			return pool.Write([]byte("some-data")) != nil
		}
		err := quick.Check(f, nil)
		Expect(err).To(BeNil())
	})
})

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
