package v2_test

import (
	"crypto/rand"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"code.cloudfoundry.org/go-loggregator/v10/rpc/loggregator_v2"
	v2 "code.cloudfoundry.org/loggregator-release/src/router/internal/server/v2"
)

var (
	gen   func() *loggregator_v2.Envelope
	s     *v2.PubSub // NopSetter subscribers — baseline for NoOp benchmarks (serial and parallel)
	sSpin *v2.PubSub // SleepSetter subscribers — realistic cost for BenchmarkDopplerRouter and BenchmarkDopplerRouterFanout
)

const numOfSubs = 100000

func TestMain(m *testing.M) {
	gen = randEnvGen()

	subscribe := func(ps *v2.PubSub, setter v2.DataSetter) {
		for i := 0; i < numOfSubs; i++ {
			ps.Subscribe(
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
	}

	s = v2.NewPubSub()
	subscribe(s, NopSetter{})

	sSpin = v2.NewPubSub()
	subscribe(sSpin, SleepSetter{})

	os.Exit(m.Run())
}

// BenchmarkDopplerRouterNoOp measures the latency of a single envelope publication through
// go-pubsub in a single thread with no parallelism. The tree holds 100k subscription
// entries across 20k distinct SourceIds (5 per SourceId); each envelope matches and
// dispatches to 5 subscribers. Uses NopSetter to isolate go-pubsub routing overhead.
func BenchmarkDopplerRouterNoOp(b *testing.B) {
	defer b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e := gen()
		s.Publish(e)
	}
}

// BenchmarkDopplerRouterParallelNoOp measures go-pubsub routing overhead with concurrent
// callers. The tree holds 100k subscription entries across 20k distinct SourceIds (5 per
// SourceId); each envelope matches 5 subscribers using NopSetter.
// Note: the router V2 server reads from a Many-to-One diode that is not safe for
// concurrent reads, so this benchmark does not reflect real production parallelism.
// See BenchmarkDopplerRouterFanout for the accurate concurrent comparison.
func BenchmarkDopplerRouterParallelNoOp(b *testing.B) {
	defer b.ReportAllocs()

	b.RunParallel(func(b *testing.PB) {
		for b.Next() {
			e := gen()
			s.Publish(e)
		}
	})
}

// BenchmarkDopplerRouter measures the latency of a single envelope publication through
// go-pubsub in a single thread, as used by the router V2 server. The tree holds 100k
// subscription entries across 20k distinct SourceIds (5 per SourceId); each envelope
// dispatches to 5 subscribers. SleepSetter sleeps 1 µs per call in an attempt to simulate realistic
// per-subscriber cost.
func BenchmarkDopplerRouter(b *testing.B) {
	defer b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e := gen()
		sSpin.Publish(e)
	}
}


// BenchmarkDopplerRouterFanout measures envelope publication throughput through
// go-pubsub with FanoutWriter dispatching to concurrent workers. The tree holds 100k
// subscription entries across 20k distinct SourceIds (5 per SourceId); each envelope
// dispatches to 5 SleepSetter subscribers (1 µs each).
func BenchmarkDopplerRouterFanout(b *testing.B) {
	// Set workers equal to GOMAXPROCS so the -cpu flag controls the degree of parallelism.
	workers := runtime.GOMAXPROCS(0)

	f := v2.NewFanoutWriter(sSpin.Publish, workers)
	f.Start()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f.Write(gen())
	}

	f.Stop()
}

type NopSetter struct{}

func (s NopSetter) Set(e *loggregator_v2.Envelope) {}

// SleepSetter simulates a non-trivial subscriber by sleeping for 1 µs per
// envelope. This makes Publish() more expensive than a no-op,
// increasing the relative benefit of parallelism in parallelism benchmarks.
type SleepSetter struct{}

func (s SleepSetter) Set(e *loggregator_v2.Envelope) {
	time.Sleep(time.Microsecond)
}

func randEnvGen() func() *loggregator_v2.Envelope {
	var s []*loggregator_v2.Envelope
	for i := 0; i < 100; i++ {
		buf := make([]byte, 10)
		_, err := rand.Read(buf) //nolint:gosec
		if err != nil {
			panic(err)
		}
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
