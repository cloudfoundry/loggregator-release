package sinkmanager

import (
	"diodes"
	"plumbing"
	"sync"

	context "golang.org/x/net/context"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/gogo/protobuf/proto"
)

type grpcRegistry struct {
	mu       sync.RWMutex
	registry map[string][]GRPCSender
}

func newGRPCRegistry() *grpcRegistry {
	return &grpcRegistry{
		registry: make(map[string][]GRPCSender),
	}
}

type GRPCSender interface {
	Send(resp *plumbing.Response) (err error)
}

func (m *SinkManager) Stream(req *plumbing.StreamRequest, d plumbing.Doppler_StreamServer) error {
	m.RegisterStream(req, d)
	<-d.Context().Done()
	return d.Context().Err()
}

func (m *SinkManager) RegisterStream(req *plumbing.StreamRequest, sender GRPCSender) {
	m.grpcStreams.mu.Lock()
	defer m.grpcStreams.mu.Unlock()
	buffSender := newBufferedGRPCSender(sender)
	m.grpcStreams.registry[req.AppID] = append(m.grpcStreams.registry[req.AppID], buffSender)
}

func (m *SinkManager) Firehose(req *plumbing.FirehoseRequest, d plumbing.Doppler_FirehoseServer) error {
	m.RegisterFirehose(req, d)
	<-d.Context().Done()
	return d.Context().Err()
}

func (m *SinkManager) RegisterFirehose(req *plumbing.FirehoseRequest, sender GRPCSender) {
	m.grpcFirehoses.mu.Lock()
	defer m.grpcFirehoses.mu.Unlock()
	buffSender := newBufferedGRPCSender(sender)
	m.grpcFirehoses.registry[req.SubID] = append(m.grpcFirehoses.registry[req.SubID], buffSender)
}

// ContainerMetrics returns the latest container metrics for a given app ID.
func (m *SinkManager) ContainerMetrics(c context.Context, req *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	resp := &plumbing.ContainerMetricsResponse{}

	for _, e := range m.LatestContainerMetrics(req.AppID) {
		mb, err := proto.Marshal(e)
		if err != nil {
			m.logger.Warnf("unable to marshal container metric: %s", err)
			continue
		}
		resp.Payload = append(resp.Payload, mb)
	}
	return resp, nil
}

// TODO:
// Error handling
// Unregister
// IsFirehoseRegistered()
// Closing go routine

type bufferedGRPCSender struct {
	sender GRPCSender
	diode  *diodes.OneToOne
}

func newBufferedGRPCSender(sender GRPCSender) *bufferedGRPCSender {
	s := &bufferedGRPCSender{
		sender: sender,
	}

	s.diode = diodes.NewOneToOne(1000, s)

	go s.run()

	return s
}

func (s *bufferedGRPCSender) Send(resp *plumbing.Response) (err error) {
	s.diode.Set(resp.Payload)
	return nil
}

func (s *bufferedGRPCSender) run() {
	for {
		payload := s.diode.Next()
		s.sender.Send(&plumbing.Response{
			Payload: payload,
		})
	}
}

func (s *bufferedGRPCSender) Alert(missed int) {
	metrics.BatchAddCounter("Diode.totalDroppedMessages", uint64(missed))
}
