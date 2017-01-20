package api

import (
	"diodes"
	"fmt"
	"log"

	clientpool "metron/clientpool/v2"
	"metron/config"
	"metron/egress"
	"metron/ingress"
	"plumbing"
	v2 "plumbing/v2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type AppV2 struct{}

func (a *AppV2) Start(conf *config.Config) {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"metron",
	)
	if err != nil {
		log.Panicf("Failed to load TLS config: %s", err)
	}

	envelopeBuffer := diodes.NewManyToOneEnvelopeV2(10000, diodes.AlertFunc(func(missed int) {
		// TODO Emit metric
		log.Printf("Dropped %d v2 envelopes", missed)
	}))

	pool := a.initializePool(conf)
	tx := egress.NewTransponder(envelopeBuffer, pool)
	go tx.Start()

	rx := ingress.NewReceiver(envelopeBuffer)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", conf.GRPC.Port)
	ingressServer := ingress.NewServer(metronAddress, rx, grpc.Creds(credentials.NewTLS(tlsConfig)))
	ingressServer.Start()
}

func (a *AppV2) initializePool(conf *config.Config) *clientpool.ClientPool {
	tlsConfig, err := plumbing.NewMutualTLSConfig(
		conf.GRPC.CertFile,
		conf.GRPC.KeyFile,
		conf.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		log.Panicf("Failed to load TLS config: %s", err)
	}

	connector := clientpool.MakeGRPCConnector(
		conf.DopplerAddr,
		conf.Zone,
		grpc.Dial,
		v2.NewDopplerIngressClient,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)

	var connManagers []clientpool.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpool.NewConnManager(connector, 10000))
	}

	return clientpool.New(connManagers...)
}
