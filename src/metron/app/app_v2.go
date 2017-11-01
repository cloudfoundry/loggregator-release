package app

import (
	"diodes"
	"fmt"
	"log"
	"math/rand"
	"metricemitter"
	"time"

	gendiodes "github.com/cloudfoundry/diodes"

	clientpool "metron/internal/clientpool/v2"
	egress "metron/internal/egress/v2"
	"metron/internal/health"
	ingress "metron/internal/ingress/v2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

type AppV2 struct {
	config         *Config
	healthRegistry *health.Registry
	clientCreds    credentials.TransportCredentials
	serverCreds    credentials.TransportCredentials
	metricClient   metricemitter.MetricClient
}

func NewV2App(
	c *Config,
	r *health.Registry,
	clientCreds credentials.TransportCredentials,
	serverCreds credentials.TransportCredentials,
	metricClient metricemitter.MetricClient,
) *AppV2 {
	return &AppV2{
		config:         c,
		healthRegistry: r,
		clientCreds:    clientCreds,
		serverCreds:    serverCreds,
		metricClient:   metricClient,
	}
}

func (a *AppV2) Start() {
	if a.serverCreds == nil {
		log.Panic("Failed to load TLS server config")
	}

	droppedMetric := a.metricClient.NewCounter("dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{"direction": "ingress"}),
	)

	envelopeBuffer := diodes.NewManyToOneEnvelopeV2(10000, gendiodes.AlertFunc(func(missed int) {
		// metric-documentation-v2: (loggregator.metron.dropped) Number of v2 envelopes
		// dropped from the metron ingress diode
		droppedMetric.Increment(uint64(missed))

		log.Printf("Dropped %d v2 envelopes", missed)
	}))

	pool := a.initializePool()
	counterAggr := egress.NewCounterAggregator(pool)
	tx := egress.NewTransponder(
		envelopeBuffer,
		counterAggr,
		a.config.Tags,
		100, time.Second,
		a.metricClient,
	)
	go tx.Start()

	metronAddress := fmt.Sprintf("127.0.0.1:%d", a.config.GRPC.Port)
	log.Printf("metron v2 API started on addr %s", metronAddress)
	rx := ingress.NewReceiver(envelopeBuffer, a.metricClient)
	ingressServer := ingress.NewServer(metronAddress, rx, grpc.Creds(a.serverCreds))
	ingressServer.Start()
}

func (a *AppV2) initializePool() *clientpool.ClientPool {
	if a.clientCreds == nil {
		log.Panic("Failed to load TLS client config")
	}

	balancers := []*clientpool.Balancer{
		clientpool.NewBalancer(a.config.DopplerAddrWithAZ),
		clientpool.NewBalancer(a.config.DopplerAddr),
	}

	kp := keepalive.ClientParameters{
		Time:                15 * time.Second,
		Timeout:             15 * time.Second,
		PermitWithoutStream: true,
	}

	fetcher := clientpool.NewSenderFetcher(
		a.healthRegistry,
		grpc.WithTransportCredentials(a.clientCreds),
		grpc.WithKeepaliveParams(kp),
	)

	connector := clientpool.MakeGRPCConnector(fetcher, balancers)

	var connManagers []clientpool.Conn
	for i := 0; i < 5; i++ {
		connManagers = append(connManagers, clientpool.NewConnManager(
			connector,
			100000+rand.Int63n(1000),
			time.Second,
		))
	}

	return clientpool.New(connManagers...)
}
