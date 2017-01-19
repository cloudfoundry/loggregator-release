package clientpool_test

import (
	"fmt"
	"metron/clientpool/v2"
	v2 "plumbing/v2"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ClientPool", func() {
	var (
		pool      *clientpool.ClientPool
		mockConns []*mockConn
	)

	BeforeEach(func() {
		var poolConns []clientpool.Conn
		mockConns = make([]*mockConn, 0)
		for i := 0; i < 5; i++ {
			conn := newMockConn()
			mockConns = append(mockConns, conn)
			poolConns = append(poolConns, conn)
		}
		pool = clientpool.New(poolConns...)
	})

	Describe("Write()", func() {
		Context("all conn managers return an error", func() {
			BeforeEach(func() {
				for _, c := range mockConns {
					c.WriteOutput.Err <- fmt.Errorf("some-error")
				}
			})

			It("returns an error", func() {
				err := pool.Write(&v2.Envelope{SourceUuid: "some-uuid"})
				Expect(err).ToNot(Succeed())
			})

			It("tries all conns before erroring", func() {
				pool.Write(&v2.Envelope{SourceUuid: "some-uuid"})

				for len(mockConns) > 0 {
					i, _ := chooseData(mockConns)
					Expect(i).ToNot(Equal(-1))
					mockConns = append(mockConns[:i], mockConns[i+1:]...)
				}
			})
		})

		Context("all conns succeed", func() {
			BeforeEach(func() {
				for _, c := range mockConns {
					c.WriteOutput.Err <- nil
				}
			})

			It("returns a nil error", func() {
				err := pool.Write(&v2.Envelope{SourceUuid: "some-uuid"})
				Expect(err).To(Succeed())
			})

			It("uses the given data once", func() {
				data := &v2.Envelope{SourceUuid: "some-uuid"}
				pool.Write(data)

				idx, msg := chooseData(mockConns)
				Expect(idx).ToNot(Equal(-1))
				Expect(msg).To(Equal(data))

				idx, _ = chooseData(mockConns)
				Expect(idx).To(Equal(-1))
			})
		})
	})
})

func chooseData(conns []*mockConn) (idx int, value *v2.Envelope) {
	var cases []reflect.SelectCase
	for _, c := range conns {
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(c.WriteInput.Data),
		})
	}
	def := reflect.SelectCase{Dir: reflect.SelectDefault}
	cases = append(cases, def)

	caseIdx, v, _ := reflect.Select(cases)
	if cases[caseIdx] == def {
		return -1, nil
	}
	return caseIdx, v.Interface().(*v2.Envelope)
}
