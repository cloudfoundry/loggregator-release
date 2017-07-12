package v1

import (
	"log"
	"sync/atomic"
	"time"

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

// DataDumper dumps Envelopes for container metrics and recent logs requests.
type DataDumper interface {
	LatestContainerMetrics(appID string) []*events.Envelope
	RecentLogsFor(appID string) []*events.Envelope
}

// DopplerServer is the GRPC server component that accepts requests for firehose
// streams, application streams, container metrics, and recent logs.
type DopplerServer struct {
	registrar           Registrar
	dumper              DataDumper
	numSubscriptions    int64
	egressMetric        *metricemitter.Counter
	egressDroppedMetric *metricemitter.Counter
	health              HealthRegistrar
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
	dumper DataDumper,
	metricClient MetricClient,
	health HealthRegistrar,
) *DopplerServer {
	egressMetric := metricClient.NewCounter("egress",
		metricemitter.WithVersion(2, 0),
	)

	egressDroppedMetric := metricClient.NewCounter("dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{"direction": "egress"}),
	)

	m := &DopplerServer{
		registrar:           registrar,
		dumper:              dumper,
		egressMetric:        egressMetric,
		egressDroppedMetric: egressDroppedMetric,
		health:              health,
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

// ContainerMetrics is called by GRPC on container metrics requests.
func (m *DopplerServer) ContainerMetrics(ctx context.Context, req *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	envelopes := m.dumper.LatestContainerMetrics(req.AppID)
	return &plumbing.ContainerMetricsResponse{
		Payload: marshalEnvelopes(envelopes),
	}, nil
}

// RecentLogs is called by GRPC on recent logs requests.
func (m *DopplerServer) RecentLogs(ctx context.Context, req *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error) {
	envelopes := m.dumper.RecentLogsFor(req.AppID)
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

		// metric-documentation-v2: (loggregator.doppler.egress) Number of
		// v1 envelopes read from a diode to be sent to subscriptions.
		m.egressMetric.Increment(1)
	}

	return sender.Context().Err()
}

// Alert logs dropped message counts to stderr.
func (m *DopplerServer) Alert(missed int) {
	// metric-documentation-v2: (loggregator.doppler.dropped) Number of
	// v1 envelopes dropped while egressing.
	m.egressDroppedMetric.Increment(uint64(missed))

	log.Printf("Dropped (egress) %d envelopes", missed)
}

func (m *DopplerServer) monitorContext(ctx context.Context, done *int64) {
	<-ctx.Done()
	atomic.StoreInt64(done, 1)
}
