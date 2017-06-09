package v2_test

import (
	"fmt"

	plumbing "code.cloudfoundry.org/loggregator/plumbing/v2"

	clientpool "code.cloudfoundry.org/loggregator/metron/internal/clientpool/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type SpyConn struct {
	err  error
	data []*plumbing.Envelope
}

func (s *SpyConn) Write(e []*plumbing.Envelope) error {
	s.data = append(s.data, e...)
	return s.err
}

var _ = Describe("ClientPool", func() {
	var (
		pool  *clientpool.ClientPool
		conns []*SpyConn
	)

	BeforeEach(func() {
		var poolConns []clientpool.Conn
		conns = nil
		for i := 0; i < 5; i++ {
			conn := &SpyConn{}
			conns = append(conns, conn)
			poolConns = append(poolConns, conn)
		}
		pool = clientpool.New(poolConns...)
	})

	Describe("Write()", func() {
		Context("with all conn managers returning an error", func() {
			BeforeEach(func() {
				for _, c := range conns {
					c.err = fmt.Errorf("some-error")
				}
			})

			It("returns an error", func() {
				err := pool.Write(nil)
				Expect(err.Error()).To(Equal("unable to write to any dopplers"))
			})

			It("tries all conns before erroring", func() {
				pool.Write([]*plumbing.Envelope{{SourceId: "some-uuid"}})

				for len(conns) > 0 {
					i, _ := chooseData(conns)
					Expect(i).ToNot(Equal(-1))
					conns = append(conns[:i], conns[i+1:]...)
				}
			})
		})

		Context("all conns succeed", func() {
			It("returns a nil error", func() {
				Expect(pool.Write(nil)).To(Succeed())
			})

			It("writes only to one connection", func() {
				Expect(pool.Write([]*plumbing.Envelope{{SourceId: "some-uuid"}})).To(Succeed())

				Expect(envelopeCount(conns)).To(Equal(1))
			})
		})
	})
})

func chooseData(conns []*SpyConn) (idx int, value *plumbing.Envelope) {
	for i, conn := range conns {
		if len(conn.data) > 0 {
			return i, conn.data[0]
		}
	}
	return -1, nil
}

func envelopeCount(conns []*SpyConn) int {
	var count int
	for _, conn := range conns {
		count += len(conn.data)
	}
	return count
}
