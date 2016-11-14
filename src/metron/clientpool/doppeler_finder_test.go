package clientpool_test

import (
	"doppler/dopplerservice"
	"errors"
	"fmt"
	"metron/clientpool"
	"testing"
	"time"

	"github.com/a8m/expect"
	"github.com/apoydence/onpar"
)

func TestFinder(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) (Expectation, *clientpool.DopplerFinder, *mockEventer) {
		eventer := newMockEventer()
		return Expectation(expect.New(t)), clientpool.NewDopplerFinder(eventer), eventer
	})

	o.Spec("it blocks if it doesn't have a doppler", func(
		t *testing.T,
		expect Expectation,
		finder *clientpool.DopplerFinder,
		mockEventer *mockEventer,
	) {
		done := make(chan bool, 100)
		go func() {
			finder.Doppler()
			done <- true
		}()

		expect(done).To.Pass(consistentlyEmpty{tries: 10})
	})

	o.Group("when eventer returns dopplers", func() {
		o.BeforeEach(func(
			t *testing.T,
			expect Expectation,
			finder *clientpool.DopplerFinder,
			mockEventer *mockEventer,
		) {
			mockEventer.NextOutput.Ret0 <- dopplerservice.Event{
				UDPDopplers: []string{"some-addr-1", "some-addr-2"},
			}
		})

		o.Spec("it returns a random doppler", func(
			t *testing.T,
			expect Expectation,
			finder *clientpool.DopplerFinder,
			mockEventer *mockEventer,
		) {
			expect(finder.Doppler).To.Pass(eventuallyReader{values: []string{
				"some-addr-1",
				"some-addr-2",
			}})
		})
	})

	o.Group("when the eventer returns no dopplers after returning dopplers", func() {
		o.BeforeEach(func(
			t *testing.T,
			expect Expectation,
			finder *clientpool.DopplerFinder,
			mockEventer *mockEventer,
		) {
			mockEventer.NextOutput.Ret0 <- dopplerservice.Event{
				UDPDopplers: []string{"some-addr-1", "some-addr-2"},
			}
		})

		o.Spec("it blocks", func(
			t *testing.T,
			expect Expectation,
			finder *clientpool.DopplerFinder,
			mockEventer *mockEventer,
		) {
			expect(finder.Doppler).To.Pass(eventuallyReader{values: []string{
				"some-addr-1",
				"some-addr-2",
			}})

			mockEventer.NextOutput.Ret0 <- dopplerservice.Event{
				UDPDopplers: []string{},
			}

			done := make(chan bool)
			go func() {
				for {
					finder.Doppler()
					done <- true
				}
			}()

			expect(done).To.Pass(eventuallyBlocks{})
		})
	})
}

type consistentlyEmpty struct {
	tries int
}

func (c consistentlyEmpty) Match(actual interface{}) error {
	f := actual.(chan bool)
	for i := 0; i < c.tries; i++ {
		if len(f) != 0 {
			return errors.New("be empty")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

type eventuallyBlocks struct {
}

func (c eventuallyBlocks) Match(actual interface{}) error {
	f := actual.(chan bool)
	for i := 0; i < 100; i++ {
		select {
		case <-f:
		case <-time.After(time.Second):
			return nil
		}
	}
	return fmt.Errorf("to block")
}

type eventuallyReader struct {
	values []string
	tries  int
}

func (e eventuallyReader) Match(actual interface{}) error {
	valueMap := make(map[string]bool)
	for _, v := range e.values {
		valueMap[v] = false
	}

	f := actual.(func() string)
	for i := 0; i < 100; i++ {
		value := f()
		_, ok := valueMap[value]
		if ok {
			delete(valueMap, value)
		}

		if len(valueMap) == 0 {
			return nil
		}

		time.Sleep(10 * time.Millisecond)
	}

	return fmt.Errorf("not all values were returned")
}
