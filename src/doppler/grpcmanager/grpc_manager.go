package grpcmanager

import (
	"diodes"
	"log"
	"plumbing"
	"sync/atomic"
	"time"

	"golang.org/x/net/context"
)

//TODO Metrics

type DataSetter interface {
	Set(data []byte)
}

type Registrar interface {
	Register(ID string, isFirehose bool, setter DataSetter) func()
}

type GrpcManager struct {
	registrar Registrar
}

type sender interface {
	Send(*plumbing.Response) error
	Context() context.Context
}

func New(registrar Registrar) *GrpcManager {
	return &GrpcManager{
		registrar: registrar,
	}
}

func (m *GrpcManager) Stream(req *plumbing.StreamRequest, sender plumbing.Doppler_StreamServer) error {
	return m.sendData(req.AppID, false, sender)
}

func (m *GrpcManager) Firehose(req *plumbing.FirehoseRequest, sender plumbing.Doppler_FirehoseServer) error {
	return m.sendData(req.SubID, true, sender)
}

func (m *GrpcManager) ContainerMetrics(ctx context.Context, req *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	return nil, nil
}

func (m *GrpcManager) RecentLogs(ctx context.Context, req *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error) {
	return nil, nil
}

func (m *GrpcManager) sendData(ID string, isFirehose bool, sender sender) error {
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

func (m *GrpcManager) Alert(missed int) {
	log.Printf("Dropped %d envelopes", missed)
}

func (m *GrpcManager) monitorContext(ctx context.Context, done *int64) {
	<-ctx.Done()
	atomic.StoreInt64(done, 1)
}
