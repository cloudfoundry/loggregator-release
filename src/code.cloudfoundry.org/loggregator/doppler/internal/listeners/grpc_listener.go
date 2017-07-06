package listeners

import (
	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/doppler/app"
	"code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v1"
	"code.cloudfoundry.org/loggregator/doppler/internal/grpcmanager/v2"
	"code.cloudfoundry.org/loggregator/doppler/internal/sinkserver/sinkmanager"
	"code.cloudfoundry.org/loggregator/healthendpoint"
	"code.cloudfoundry.org/loggregator/metricemitter"
	plumbingv1 "code.cloudfoundry.org/loggregator/plumbing"
	plumbingv2 "code.cloudfoundry.org/loggregator/plumbing/v2"
	"fmt"
	"log"
	"net"

	"github.com/cloudfoundry/dropsonde/metricbatcher"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type GRPCListener struct {
	listener net.Listener
	server   *grpc.Server
}

func NewGRPCListener(
	reg v1.Registrar,
	sinkmanager *sinkmanager.SinkManager,
	conf app.GRPC,
	envelopeBuffer *diodes.ManyToOneEnvelope,
	batcher *metricbatcher.MetricBatcher,
	metricClient metricemitter.MetricClient,
	health *healthendpoint.Registrar,
) (*GRPCListener, error) {
	tlsConfig, err := plumbingv1.NewMutualTLSConfig(
		conf.CertFile,
		conf.KeyFile,
		conf.CAFile,
		"doppler",
	)
	if err != nil {
		return nil, err
	}
	transportCreds := credentials.NewTLS(tlsConfig)

	log.Printf("Listening for GRPC connections on %d", conf.Port)
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.Port))

	if err != nil {
		log.Printf("Failed to start listener (port=%d) for gRPC: %s", conf.Port, err)
		return nil, err
	}
	grpcServer := grpc.NewServer(
		grpc.Creds(transportCreds),
		grpc.MaxMsgSize(64*1024),
	)

	// v1 ingress
	plumbingv1.RegisterDopplerIngestorServer(
		grpcServer,
		v1.NewIngestorServer(envelopeBuffer, batcher, health),
	)
	// v1 egress
	plumbingv1.RegisterDopplerServer(
		grpcServer,
		v1.NewDopplerServer(reg, sinkmanager, metricClient, health),
	)

	// v2 ingress
	plumbingv2.RegisterDopplerIngressServer(
		grpcServer,
		v2.NewIngressServer(envelopeBuffer, batcher, metricClient, health),
	)

	return &GRPCListener{
		listener: grpcListener,
		server:   grpcServer,
	}, nil
}

func (g *GRPCListener) Start() {
	log.Printf("Starting gRPC server on %s", g.listener.Addr().String())
	if err := g.server.Serve(g.listener); err != nil {
		log.Fatalf("Failed to start gRPC server: %s", err)
	}
}
