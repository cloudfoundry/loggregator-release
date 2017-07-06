package egress_test

import (
	"net"
	"sync"

	"code.cloudfoundry.org/loggregator/rlp/internal/egress"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LimitListener", func() {
	var (
		l               *spyListener
		c               *spyConn
		limitedListener net.Listener
	)

	BeforeEach(func() {
		c = &spyConn{}
		l = &spyListener{c: c}

		limitedListener = egress.LimitListener(l, 1)
	})

	It("closes any connection that surpasses the limit", func() {
		_, err := limitedListener.Accept()
		Expect(err).ToNot(HaveOccurred())
		Expect(l.Called()).To(Equal(1))
		Expect(c.Called()).To(Equal(0))

		done := make(chan struct{})
		go func() {
			defer close(done)
			limitedListener.Accept()
		}()
		Consistently(done).ShouldNot(BeClosed())
		Eventually(c.Called).Should(BeNumerically(">=", 1))
	})
})

type spyListener struct {
	net.Listener
	c net.Conn

	mu     sync.Mutex
	called int
}

func (s *spyListener) Accept() (net.Conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called++
	return s.c, nil
}

func (s *spyListener) Close() error {
	return nil
}

func (s *spyListener) Called() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.called
}

type spyConn struct {
	net.Conn
	mu     sync.Mutex
	called int
}

func (s *spyConn) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called++
	return nil
}

func (s *spyConn) Called() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.called
}
