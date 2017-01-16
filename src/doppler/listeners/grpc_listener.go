package listeners

import (
	"diodes"
	"doppler/config"
	"doppler/grpcmanager/v1"
	"doppler/sinkserver/sinkmanager"
	"fmt"
	"log"
	"net"
	"plumbing"

	"github.com/cloudfoundry/dropsonde/metricbatcher"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type GRPCListener struct {
	listener net.Listener
	server   *grpc.Server
}

func NewGRPCListener(
	router *v1.Router,
	sinkmanager *sinkmanager.SinkManager,
	conf config.GRPC,
	envelopeBuffer *diodes.ManyToOneEnvelope,
	batcher *metricbatcher.MetricBatcher,
) (*GRPCListener, error) {
	grpcManager := v1.New(router, sinkmanager)

	tlsConfig, err := plumbing.NewMutualTLSConfig(
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
	grpcServer := grpc.NewServer(grpc.Creds(transportCreds))
	grpcIngestorManager := v1.NewIngestor(envelopeBuffer, batcher)

	plumbing.RegisterDopplerIngestorServer(grpcServer, grpcIngestorManager)
	plumbing.RegisterDopplerServer(grpcServer, grpcManager)

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
