package v1

import (
	"log"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/batching"
	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"golang.org/x/net/context"
)

const (
	metricsInterval = time.Second
)

type HealthRegistrar interface {
	Inc(name string)
	Dec(name string)
}

// Registrar registers stream and firehose DataSetters to accept reads.
type Registrar interface {
	Register(req *plumbing.SubscriptionRequest, setter DataSetter) func()
}

// DataSetter accepts writes of marshalled data.
type DataSetter interface {
	Set(data []byte)
}

// EnvelopeStore returns Envelopes for container metrics and recent logs requests.
type EnvelopeStore interface {
	LatestContainerMetrics(appID string) []*events.Envelope
	RecentLogsFor(appID string) []*events.Envelope
}

// DopplerServer is the GRPC server component that accepts requests for firehose
// streams, application streams, container metrics, and recent logs.
type DopplerServer struct {
	registrar        Registrar
	envelopeStore    EnvelopeStore
	numSubscriptions int64
	egressMetric     *metricemitter.Counter
	health           HealthRegistrar
	batchInterval    time.Duration
	batchSize        uint
}

type sender interface {
	Send(*plumbing.Response) error
	Context() context.Context
}

// MetricClient creates new CounterMetrics to be emitted periodically.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
}

// NewDopplerServer creates a new DopplerServer.
func NewDopplerServer(
	registrar Registrar,
	envelopeStore EnvelopeStore,
	metricClient MetricClient,
	health HealthRegistrar,
	batchInverval time.Duration,
	batchSize uint,
) *DopplerServer {
	// metric-documentation-v2: (loggregator.doppler.egress) Number of
	// v1 envelopes read from a diode to be sent to subscriptions.
	egressMetric := metricClient.NewCounter(
		"egress",
		metricemitter.WithVersion(2, 0),
	)

	m := &DopplerServer{
		registrar:     registrar,
		envelopeStore: envelopeStore,
		egressMetric:  egressMetric,
		health:        health,
		batchInterval: batchInverval,
		batchSize:     batchSize,
	}

	go m.emitMetrics()

	return m
}

// Subscribe is called by GRPC on stream requests.
func (m *DopplerServer) Subscribe(req *plumbing.SubscriptionRequest, sender plumbing.Doppler_SubscribeServer) error {
	atomic.AddInt64(&m.numSubscriptions, 1)
	defer atomic.AddInt64(&m.numSubscriptions, -1)
	m.health.Inc("subscriptionCount")
	defer m.health.Dec("subscriptionCount")

	return m.sendData(req, sender)
}

// BatchSubscribe is called by GRPC on stream batch requests.
func (m *DopplerServer) BatchSubscribe(req *plumbing.SubscriptionRequest, sender plumbing.Doppler_BatchSubscribeServer) error {
	atomic.AddInt64(&m.numSubscriptions, 1)
	defer atomic.AddInt64(&m.numSubscriptions, -1)
	m.health.Inc("subscriptionCount")
	defer m.health.Dec("subscriptionCount")

	return m.sendBatchData(req, sender)
}

// ContainerMetrics is called by GRPC on container metrics requests.
func (m *DopplerServer) ContainerMetrics(ctx context.Context, req *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	envelopes := m.envelopeStore.LatestContainerMetrics(req.AppID)
	return &plumbing.ContainerMetricsResponse{
		Payload: marshalEnvelopes(envelopes),
	}, nil
}

// RecentLogs is called by GRPC on recent logs requests.
func (m *DopplerServer) RecentLogs(ctx context.Context, req *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error) {
	envelopes := m.envelopeStore.RecentLogsFor(req.AppID)
	return &plumbing.RecentLogsResponse{
		Payload: marshalEnvelopes(envelopes),
	}, nil
}

func (m *DopplerServer) emitMetrics() {
	for range time.Tick(metricsInterval) {
		// metrics:v1 (grpcManager.subscriptions) Number of v1 egress gRPC
		// subscriptions
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

func (m *DopplerServer) sendData(req *plumbing.SubscriptionRequest, sender sender) error {
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

		err := sender.Send(&plumbing.Response{Payload: data})
		if err != nil {
			return err
		}

		m.egressMetric.Increment(1)
	}

	return sender.Context().Err()
}

type batchWriter struct {
	sender       plumbing.Doppler_BatchSubscribeServer
	errStream    chan<- error
	egressMetric *metricemitter.Counter
}

func (b *batchWriter) Write(batch [][]byte) {
	err := b.sender.Send(&plumbing.BatchResponse{Payload: batch})
	if err != nil {
		b.errStream <- err
	}
	b.egressMetric.Increment(uint64(len(batch)))
}

func (m *DopplerServer) sendBatchData(req *plumbing.SubscriptionRequest, sender plumbing.Doppler_BatchSubscribeServer) error {
	d := diodes.NewOneToOne(1000, m)
	cleanup := m.registrar.Register(req, d)
	defer cleanup()

	errStream := make(chan error, 1)
	batcher := batching.NewByteBatcher(
		int(m.batchSize),
		m.batchInterval,
		&batchWriter{
			sender:       sender,
			errStream:    errStream,
			egressMetric: m.egressMetric,
		},
	)

	for {
		select {
		case <-sender.Context().Done():
			return sender.Context().Err()
		case err := <-errStream:
			return err
		default:
			data, ok := d.TryNext()
			if !ok {
				batcher.Flush()
				time.Sleep(10 * time.Millisecond)
				continue
			}

			batcher.Write(data)

			// TODO Send loggregator.doppler.egress metric
		}
	}
}

// Alert logs dropped message counts to stderr.
func (m *DopplerServer) Alert(missed int) {
	log.Printf("Dropped (egress) %d envelopes", missed)
}

func (m *DopplerServer) monitorContext(ctx context.Context, done *int64) {
	<-ctx.Done()
	atomic.StoreInt64(done, 1)
}
