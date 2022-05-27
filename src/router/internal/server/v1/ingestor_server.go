package v1

import (
	"context"
	"io"
	"log"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/go-loggregator/v8/conversion"
	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type IngestorServer struct {
	v1Buf         *diodes.ManyToOneEnvelope
	v2Buf         *diodes.ManyToOneEnvelopeV2
	ingressMetric *metricemitter.Counter
}

type IngestorGRPCServer interface {
	plumbing.DopplerIngestor_PusherServer
}

func NewIngestorServer(
	v1Buf *diodes.ManyToOneEnvelope,
	v2Buf *diodes.ManyToOneEnvelopeV2,
	ingressMetric *metricemitter.Counter,
) *IngestorServer {
	return &IngestorServer{
		v1Buf:         v1Buf,
		v2Buf:         v2Buf,
		ingressMetric: ingressMetric,
	}
}

func (i *IngestorServer) Pusher(pusher plumbing.DopplerIngestor_PusherServer) error {
	var done int64
	context := pusher.Context()
	go i.monitorContext(context, &done)

	for {
		if atomic.LoadInt64(&done) > 0 {
			return context.Err()
		}

		envelopeData, err := pusher.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		env := &events.Envelope{}
		err = proto.Unmarshal(envelopeData.Payload, env)
		if err != nil {
			log.Printf("Received bad envelope: %s", err)
			continue
		}

		v2e := conversion.ToV2(env, true)
		i.v1Buf.Set(env)
		i.v2Buf.Set(v2e)

		i.ingressMetric.Increment(1)
	}
	return nil
}

func (i *IngestorServer) monitorContext(ctx context.Context, done *int64) {
	<-ctx.Done()
	atomic.StoreInt64(done, 1)
}
