package grpcmanager

import (
	"diodes"
	"log"
	"plumbing"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"golang.org/x/net/context"
)

const (
	metricsInterval = time.Second
)

// Registrar registers stream and firehose DataSetters to accept reads.
type Registrar interface {
	Register(req *plumbing.SubscriptionRequest, setter DataSetter) func()
}

// DataSetter accepts writes of marshalled data.
type DataSetter interface {
	Set(data []byte)
}

// DataDumper dumps Envelopes for container metrics and recent logs requests.
type DataDumper interface {
	LatestContainerMetrics(appID string) []*events.Envelope
	RecentLogsFor(appID string) []*events.Envelope
}

// GRPCManager is the GRPC server component that accepts requests for firehose
// streams, application streams, container metrics, and recent logs.
type GRPCManager struct {
	registrar        Registrar
	dumper           DataDumper
	numSubscriptions int64
}

type sender interface {
	Send(*plumbing.Response) error
	Context() context.Context
}

// New creates a new GRPCManager.
func New(registrar Registrar, dumper DataDumper) *GRPCManager {
	m := &GRPCManager{
		registrar: registrar,
		dumper:    dumper,
	}

	go m.emitMetrics()
	return m
}

// Subscribe is called by GRPC on stream requests.
func (m *GRPCManager) Subscribe(req *plumbing.SubscriptionRequest, sender plumbing.Doppler_SubscribeServer) error {
	atomic.AddInt64(&m.numSubscriptions, 1)
	defer atomic.AddInt64(&m.numSubscriptions, -1)

	return m.sendData(req, sender)
}

// ContainerMetrics is called by GRPC on container metrics requests.
func (m *GRPCManager) ContainerMetrics(ctx context.Context, req *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	envelopes := m.dumper.LatestContainerMetrics(req.AppID)
	return &plumbing.ContainerMetricsResponse{
		Payload: marshalEnvelopes(envelopes),
	}, nil
}

// RecentLogs is called by GRPC on recent logs requests.
func (m *GRPCManager) RecentLogs(ctx context.Context, req *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error) {
	envelopes := m.dumper.RecentLogsFor(req.AppID)
	return &plumbing.RecentLogsResponse{
		Payload: marshalEnvelopes(envelopes),
	}, nil
}

func (m *GRPCManager) emitMetrics() {
	for range time.Tick(metricsInterval) {
		metrics.SendValue("grpcManager.subscriptions", float64(atomic.LoadInt64(&m.numSubscriptions)), "subscriptions")
	}
}

func marshalEnvelopes(envelopes []*events.Envelope) [][]byte {
	var marshalled [][]byte
	for _, env := range envelopes {
		bts, err := proto.Marshal(env)
		if err != nil {
			continue
		}
		marshalled = append(marshalled, bts)
	}
	return marshalled
}

func (m *GRPCManager) sendData(req *plumbing.SubscriptionRequest, sender sender) error {
	d := diodes.NewOneToOne(1000, m)
	cleanup := m.registrar.Register(req, d)
	defer cleanup()

	var done int64
	go m.monitorContext(sender.Context(), &done)

	for {
		if atomic.LoadInt64(&done) > 0 {
			break
		}

		data, ok := d.TryNext()
		if !ok {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		err := sender.Send(&plumbing.Response{
			Payload: data,
		})

		if err != nil {
			return err
		}
	}

	return sender.Context().Err()
}

// Alert logs dropped message counts to stderr.
func (m *GRPCManager) Alert(missed int) {
	log.Printf("Dropped %d envelopes", missed)
}

func (m *GRPCManager) monitorContext(ctx context.Context, done *int64) {
	<-ctx.Done()
	atomic.StoreInt64(done, 1)
}
