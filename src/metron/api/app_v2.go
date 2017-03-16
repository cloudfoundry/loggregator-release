package api

import (
	"diodes"
	"fmt"
	"log"
	"math/rand"
	"metric"
	"time"

	clientpool "metron/clientpool/v2"
	egress "metron/egress/v2"
	ingress "metron/ingress/v2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type AppV2 struct {
	config      *Config
	clientCreds credentials.TransportCredentials
	serverCreds credentials.TransportCredentials
}

func NewV2App(
	c *Config,
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
		// metric:v2 (loggregator.metron.dropped) Number of v2 envelopes
		// droppred from the metron ingress diode
		metric.IncCounter("dropped",
			metric.WithIncrement(uint64(missed)),
			metric.WithVersion(2, 0),
			metric.WithTag("direction", "ingress"),
		)
		log.Printf("Dropped %d v2 envelopes", missed)
	}))

	pool := a.initializePool()
	counterAggr := egress.New(pool)
	tx := egress.NewTransponder(envelopeBuffer, counterAggr)
	go tx.Start()

	metronAddress := fmt.Sprintf("127.0.0.1:%d", a.config.GRPC.Port)
	log.Printf("metron v2 API started on addr %s", metronAddress)
	rx := ingress.NewReceiver(envelopeBuffer)
	ingressServer := ingress.NewServer(metronAddress, rx, grpc.Creds(a.serverCreds))
	ingressServer.Start()
}

func (a *AppV2) initializePool() *clientpool.ClientPool {
	if a.clientCreds == nil {
		log.Panic("Failed to load TLS client config")
	}

	balancers := []*clientpool.Balancer{
		clientpool.NewBalancer(fmt.Sprintf("%s.%s", a.config.Zone, a.config.DopplerAddr)),
		clientpool.NewBalancer(a.config.DopplerAddr),
	}

	fetcher := clientpool.NewSenderFetcher(
		grpc.WithTransportCredentials(a.clientCreds),
	)

	connector := clientpool.MakeGRPCConnector(fetcher, balancers)

	var connManagers []clientpool.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpool.NewConnManager(
			connector,
			10000+rand.Int63n(1000),
			time.Second,
		))
	}

	return clientpool.New(connManagers...)
}
