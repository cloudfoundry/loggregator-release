package plumbing_test

import (
	"io/ioutil"
	"log"
	"net"
	"testing"
	"time"

	"github.com/apoydence/eachers/testhelpers"

	"code.cloudfoundry.org/loggregator/dopplerservice"
	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	"code.cloudfoundry.org/loggregator/plumbing"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

func BenchmarkGRPCConnectorParallel(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))

	spyDopplerA := newSpyDoppler()
	spyDopplerB := newSpyDoppler()

	pool := plumbing.NewPool(20, grpc.WithInsecure())
	finder := newMockFinder()
	finder.NextOutput.Ret0 <- dopplerservice.Event{
		GRPCDopplers: []string{
			spyDopplerA.addr.String(),
			spyDopplerB.addr.String(),
		},
	}
	metricClient := testhelper.NewMetricClient()
	batcher := newMockMetaMetricBatcher()
	chainer := newMockBatchCounterChainer()
	testhelpers.AlwaysReturn(batcher.BatchCounterOutput, chainer)
	testhelpers.AlwaysReturn(chainer.SetTagOutput, chainer)
	connector := plumbing.NewGRPCConnector(5, pool, finder, batcher, metricClient)

	time.Sleep(2 * time.Second)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		var data [][]byte

		for pb.Next() {
			data = connector.ContainerMetrics(context.Background(), "an-app-id")
		}

		_ = data
	})
}

type spyDoppler struct {
	addr       net.Addr
	grpcServer *grpc.Server
}

func newSpyDoppler() *spyDoppler {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	spy := &spyDoppler{
		addr:       lis.Addr(),
		grpcServer: grpc.NewServer(),
	}

	plumbing.RegisterDopplerServer(spy.grpcServer, spy)

	go func() {
		log.Println(spy.grpcServer.Serve(lis))
	}()

	return spy
}

func (m *spyDoppler) Subscribe(r *plumbing.SubscriptionRequest, s plumbing.Doppler_SubscribeServer) error {
	<-s.Context().Done()
	return nil
}

func (m *spyDoppler) BatchSubscribe(r *plumbing.SubscriptionRequest, s plumbing.Doppler_BatchSubscribeServer) error {
	<-s.Context().Done()
	return nil
}

func (m *spyDoppler) ContainerMetrics(context.Context, *plumbing.ContainerMetricsRequest) (*plumbing.ContainerMetricsResponse, error) {
	return &plumbing.ContainerMetricsResponse{
		Payload: [][]byte{},
	}, nil
}

func (m *spyDoppler) RecentLogs(context.Context, *plumbing.RecentLogsRequest) (*plumbing.RecentLogsResponse, error) {
	return &plumbing.RecentLogsResponse{
		Payload: [][]byte{},
	}, nil
}

func (m *spyDoppler) Stop() {
	m.grpcServer.Stop()
}
