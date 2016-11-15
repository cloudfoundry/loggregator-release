//go:generate hel

package clientpool_test

import (
	"fmt"
	"metron/clientpool"
	"reflect"
	"testing"

	"github.com/a8m/expect"
	"github.com/apoydence/onpar"
)

func TestClientPool(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) (Expectation, *clientpool.ClientPool, []*mockConn) {
		var conns []*mockConn
		var poolConns []clientpool.Conn
		for i := 0; i < 5; i++ {
			conn := newMockConn()
			conns = append(conns, conn)
			poolConns = append(poolConns, conn)
		}
		return expect.New(t), clientpool.New(poolConns...), conns
	})

	o.Group("Write()", func() {
		o.Group("all conn managers return an error", func() {
			o.BeforeEach(func(t *testing.T, expect Expectation, pool *clientpool.ClientPool, mockConns []*mockConn) {
				for _, c := range mockConns {
					c.WriteOutput.Err <- fmt.Errorf("some-error")
				}
			})

			o.Spec("it returns an error", func(t *testing.T, expect Expectation, pool *clientpool.ClientPool, mockConns []*mockConn) {
				err := pool.Write([]byte("some-data"))
				expect(err).Not.To.Be.Nil()
			})

			o.Spec("it tries all conns before erroring", func(t *testing.T, expect Expectation, pool *clientpool.ClientPool, mockConns []*mockConn) {
				pool.Write([]byte("some-data"))

				for len(mockConns) > 0 {
					i, _ := chooseData(mockConns)
					expect(i).Not.To.Equal(-1)
					mockConns = append(mockConns[:i], mockConns[i+1:]...)
				}
			})
		})

		o.Group("all conns succeed", func() {
			o.BeforeEach(func(t *testing.T, expect Expectation, pool *clientpool.ClientPool, mockConns []*mockConn) {
				for _, c := range mockConns {
					c.WriteOutput.Err <- nil
				}
			})

			o.Spec("it returns a nil error", func(t *testing.T, expect Expectation, pool *clientpool.ClientPool, mockConns []*mockConn) {
				err := pool.Write([]byte("some-data"))
				expect(err).To.Be.Nil()
			})

			o.Spec("it uses the given data once", func(t *testing.T, expect Expectation, pool *clientpool.ClientPool, mockConns []*mockConn) {
				data := []byte("some-data")
				pool.Write(data)

				idx, msg := chooseData(mockConns)
				expect(idx).Not.To.Equal(-1)
				expect(msg).To.Equal(data)

				idx, _ = chooseData(mockConns)
				expect(idx).To.Equal(-1)
			})
		})
	})
}

func chooseData(conns []*mockConn) (idx int, value []byte) {
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
	return caseIdx, v.Interface().([]byte)
}
