// dummymetron: a program that accepts envelopes via UDP (v1) and gRPC (v2).
//
package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/plumbing/v2"
)

var (
	udpPort  = flag.Int("udp-port", 12345, "port to use to listen for UDP (v1)")
	grpcPort = flag.Int("grpc-port", 12345, "port to use to listen for gRPC (v2)")
	certFile = flag.String("cert", "", "cert to use to listen for gRPC")
	keyFile  = flag.String("key", "", "key to use to listen for gRPC")
	caFile   = flag.String("ca", "", "ca cert to use to listen for gRPC")
)

func main() {
	flag.Parse()

	// v1
	{
		connection, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", *udpPort))
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Listening on %s", connection.LocalAddr().String())
		go func() {
			b := make([]byte, 1024)
			for {
				_, _, err := connection.ReadFrom(b)
				if err != nil {
					log.Print(err)
				}
			}
		}()
	}

	// v2
	{
		tlsConfig, err := plumbing.NewServerMutualTLSConfig(
			*certFile,
			*keyFile,
			*caFile,
		)
		if err != nil {
			log.Fatal(err)
		}
		transportCreds := credentials.NewTLS(tlsConfig)

		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
		if err != nil {
			log.Fatal(err)
		}
		grpcServer := grpc.NewServer(grpc.Creds(transportCreds))
		loggregator_v2.RegisterIngressServer(grpcServer, &Server{})
		log.Printf("Starting gRPC server on %s", listener.Addr().String())
		log.Fatal(grpcServer.Serve(listener))
	}
}

type Server struct{}

func (s *Server) Sender(server loggregator_v2.Ingress_SenderServer) error {
	for {
		_, err := server.Recv()
		if err != nil {
			log.Print(err)
			return nil
		}
	}
}

func (s *Server) BatchSender(server loggregator_v2.Ingress_BatchSenderServer) error {
	for {
		_, err := server.Recv()
		if err != nil {
			log.Print(err)
			return nil
		}
	}
}
