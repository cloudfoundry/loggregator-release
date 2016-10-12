package grpcmanager

import (
	"diodes"
	"log"
	"plumbing"
	"sync/atomic"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"

	"golang.org/x/net/context"
)

// Registrar registers stream and firehose DataSetters to accept reads.
type Registrar interface {
	Register(ID string, isFirehose bool, setter DataSetter) func()
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
	registrar Registrar
	dumper    DataDumper
}

type sender interface {
	Send(*plumbing.Response) error
	Context() context.Context
}

// New creates a new GRPCManager.
func New(registrar Registrar, dumper DataDumper) *GRPCManager {
	return &GRPCManager{
		registrar: registrar,
		dumper:    dumper,
	}
}

// Stream is called by GRPC on application stream requests.
func (m *GRPCManager) Stream(req *plumbing.StreamRequest, sender plumbing.Doppler_StreamServer) error {
	return m.sendData(req.AppID, false, sender)
}

// Firehose is called by GRPC on firehose stream requests.
func (m *GRPCManager) Firehose(req *plumbing.FirehoseRequest, sender plumbing.Doppler_FirehoseServer) error {
	return m.sendData(req.SubID, true, sender)
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

func (m *GRPCManager) sendData(ID string, isFirehose bool, sender sender) error {
	d := diodes.NewOneToOne(1000, m)
	cleanup := m.registrar.Register(ID, isFirehose, d)
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
