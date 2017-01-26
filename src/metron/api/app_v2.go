package api

import (
	"diodes"
	"fmt"
	"log"
	"math/rand"

	clientpool "metron/clientpool/v2"
	"metron/config"
	"metron/egress"
	"metron/ingress"
	v2 "plumbing/v2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type AppV2 struct {
	config      *config.Config
	clientCreds credentials.TransportCredentials
	serverCreds credentials.TransportCredentials
}

func NewV2App(
	c *config.Config,
	clientCreds credentials.TransportCredentials,
	serverCreds credentials.TransportCredentials,
) *AppV2 {
	return &AppV2{
		config:      c,
		clientCreds: clientCreds,
		serverCreds: serverCreds,
	}
}

func (a *AppV2) Start() {
	if a.serverCreds == nil {
		log.Panic("Failed to load TLS server config")
	}

	envelopeBuffer := diodes.NewManyToOneEnvelopeV2(10000, diodes.AlertFunc(func(missed int) {
		// TODO Emit metric
		log.Printf("Dropped %d v2 envelopes", missed)
	}))

	pool := a.initializePool()
	tx := egress.NewTransponder(envelopeBuffer, pool)
	go tx.Start()

	rx := ingress.NewReceiver(envelopeBuffer)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", a.config.GRPC.Port)
	ingressServer := ingress.NewServer(metronAddress, rx, grpc.Creds(a.serverCreds))
	ingressServer.Start()
}

func (a *AppV2) initializePool() *clientpool.ClientPool {
	if a.clientCreds == nil {
		log.Panic("Failed to load TLS client config")
	}

	connector := clientpool.MakeGRPCConnector(
		a.config.DopplerAddr,
		a.config.Zone,
		grpc.Dial,
		v2.NewDopplerIngressClient,
		grpc.WithTransportCredentials(a.clientCreds),
	)

	var connManagers []clientpool.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpool.NewConnManager(connector, 10000+rand.Int63n(1000)))
	}

	return clientpool.New(connManagers...)
}
