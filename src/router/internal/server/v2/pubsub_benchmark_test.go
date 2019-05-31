package v2_test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/router/internal/server/v2"
)

var (
	gen func() *loggregator_v2.Envelope
	s   *v2.PubSub
)

const numOfSubs = 100000

func TestMain(m *testing.M) {
	gen = randEnvGen()

	s = v2.NewPubSub()
	setter := NopSetter{}

	for i := 0; i < numOfSubs; i++ {
		s.Subscribe(
			&loggregator_v2.EgressBatchRequest{
				Selectors: []*loggregator_v2.Selector{
					{
						SourceId: fmt.Sprintf("%d", i%20000),
						Message: &loggregator_v2.Selector_Log{
							Log: &loggregator_v2.LogSelector{},
						},
					},
				},
			},
			setter,
		)
	}

	os.Exit(m.Run())
}

func BenchmarkDopplerRouter(b *testing.B) {
	defer b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e := gen()
		s.Publish(e)
	}
}

type NopSetter struct{}

var data []byte

func (s NopSetter) Set(e *loggregator_v2.Envelope) {}

func randEnvGen() func() *loggregator_v2.Envelope {
	var s []*loggregator_v2.Envelope
	for i := 0; i < 100; i++ {
		buf := make([]byte, 10)
		rand.Read(buf)
		s = append(s, benchBuildLog(fmt.Sprintf("%d", i%20000), buf))
	}

	var i int
	return func() *loggregator_v2.Envelope {
		i++
		return s[i%len(s)]
	}
}

func benchBuildLog(appID string, payload []byte) *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		SourceId:  appID,
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: payload,
			},
		},
	}
}
