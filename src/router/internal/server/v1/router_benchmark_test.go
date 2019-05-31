package v1_test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/router/internal/server/v1"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

var (
	gen func() *events.Envelope
	r   *v1.Router
)

const numOfSubs = 100000

func TestMain(m *testing.M) {
	gen = randEnvGen()

	r = v1.NewRouter()
	setter := NopSetter{}

	for i := 0; i < numOfSubs; i++ {
		r.Register(&plumbing.SubscriptionRequest{Filter: &plumbing.Filter{
			AppID:   fmt.Sprintf("%d", i%20000),
			Message: &plumbing.Filter_Log{&plumbing.LogFilter{}},
		}}, setter)
	}

	os.Exit(m.Run())
}

func BenchmarkDopplerRouter(b *testing.B) {
	defer b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		e := gen()
		r.SendTo(e.GetLogMessage().GetAppId(), e)
	}
}

type NopSetter struct{}

func (s NopSetter) Set(data []byte) {}

func randEnvGen() func() *events.Envelope {
	var s []*events.Envelope
	for i := 0; i < 100; i++ {
		buf := make([]byte, 10)
		rand.Read(buf)
		s = append(s, buildLog(fmt.Sprintf("%d", i%20000), buf))
	}

	var i int
	return func() *events.Envelope {
		i++
		return s[i%len(s)]
	}
}

func buildLog(appID string, payload []byte) *events.Envelope {
	return &events.Envelope{
		Origin:     proto.String("some-origin"),
		EventType:  events.Envelope_LogMessage.Enum(),
		Deployment: proto.String("some-deployment"),
		Job:        proto.String("some-job"),
		Index:      proto.String("some-index"),
		Ip:         proto.String("some-ip"),
		Timestamp:  proto.Int64(time.Now().UnixNano()),
		LogMessage: &events.LogMessage{
			Message:        payload,
			MessageType:    events.LogMessage_OUT.Enum(),
			Timestamp:      proto.Int64(time.Now().UnixNano()),
			AppId:          proto.String(appID),
			SourceType:     proto.String("test-source-type"),
			SourceInstance: proto.String("test-source-instance"),
		},
	}
}
