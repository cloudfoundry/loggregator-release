package v1

import (
	"context"
	"io"
	"log"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
)

type IngestorServer struct {
	sender  MessageSender
	batcher Batcher
	health  HealthRegistrar
}

type Batcher interface {
	BatchCounter(name string) metricbatcher.BatchCounterChainer
}

type MessageSender interface {
	Set(*events.Envelope)
}

type IngestorGRPCServer interface {
	plumbing.DopplerIngestor_PusherServer
}

func NewIngestorServer(
	sender MessageSender,
	batcher Batcher,
	health HealthRegistrar,
) *IngestorServer {

	return &IngestorServer{
		sender:  sender,
		batcher: batcher,
		health:  health,
	}
}

func (i *IngestorServer) Pusher(pusher plumbing.DopplerIngestor_PusherServer) error {
	i.health.Inc("ingressStreamCount")
	defer i.health.Dec("ingressStreamCount")

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
		// metric-documentation-v1: (listeners.receivedEnvelopes) Number of received
		// envelopes in v1 ingress server
		i.batcher.BatchCounter("listeners.receivedEnvelopes").
			SetTag("protocol", "grpc").
			SetTag("event_type", env.GetEventType().String()).
			Increment()
		i.sender.Set(env)

		// metric-documentation-v1: (listeners.totalReceivedMessageCount) Total
		// number of messages received by doppler.
		i.batcher.BatchCounter("listeners.totalReceivedMessageCount").
			Increment()
	}
	return nil
}

func (i *IngestorServer) monitorContext(ctx context.Context, done *int64) {
	<-ctx.Done()
	atomic.StoreInt64(done, 1)
}
