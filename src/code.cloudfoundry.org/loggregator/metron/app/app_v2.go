package app

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/diodes"
	"code.cloudfoundry.org/loggregator/metricemitter"

	gendiodes "github.com/cloudfoundry/diodes"

	clientpool "code.cloudfoundry.org/loggregator/metron/internal/clientpool/v2"
	egress "code.cloudfoundry.org/loggregator/metron/internal/egress/v2"
	"code.cloudfoundry.org/loggregator/metron/internal/health"
	ingress "code.cloudfoundry.org/loggregator/metron/internal/ingress/v2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type AppV2 struct {
	addr           net.Addr
	ingressServer  *ingress.Server
	envelopeBuffer *diodes.ManyToOneEnvelopeV2
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
	if serverCreds == nil {
		log.Panic("Failed to load TLS server config: provided nil server creds")
	}
	if clientCreds == nil {
		log.Panic("Failed to load TLS client config: provided nil client creds")
	}

	droppedMetric := metricClient.NewCounterMetric("dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{"direction": "ingress"}),
	)
	envelopeBuffer := diodes.NewManyToOneEnvelopeV2(10000, gendiodes.AlertFunc(func(missed int) {
		// metric-documentation-v2: (loggregator.metron.dropped) Number of v2 envelopes
		// dropped from the metron ingress diode
		droppedMetric.Increment(uint64(missed))

		log.Printf("Dropped %d v2 envelopes", missed)
	}))

	rx := ingress.NewReceiver(envelopeBuffer, metricClient)
	metronAddress := fmt.Sprintf("127.0.0.1:%d", c.GRPC.Port)
	ingressServer := ingress.NewServer(metronAddress, rx, grpc.Creds(serverCreds))
	addr := ingressServer.Addr()

	return &AppV2{
		addr:           addr,
		ingressServer:  ingressServer,
		envelopeBuffer: envelopeBuffer,
		config:         c, // remove from state?
		healthRegistry: r,
		clientCreds:    clientCreds,
		serverCreds:    serverCreds,
		metricClient:   metricClient,
	}
}

func (a *AppV2) Addr() net.Addr {
	return a.addr
}

func (a *AppV2) Start() {
	pool := a.initializePool()
	counterAggr := egress.NewCounterAggregator(pool)
	tx := egress.NewTransponder(
		a.envelopeBuffer,
		counterAggr,
		a.config.Tags,
		100, time.Second,
		a.metricClient,
	)
	go tx.Start()

	log.Printf("metron v2 API started on addr %s", a.addr)
	a.ingressServer.Start()
}

func (a *AppV2) initializePool() *clientpool.ClientPool {
	balancers := []*clientpool.Balancer{
		clientpool.NewBalancer(fmt.Sprintf("%s.%s", a.config.Zone, a.config.DopplerAddr)),
		clientpool.NewBalancer(a.config.DopplerAddr),
	}

	fetcher := clientpool.NewSenderFetcher(
		a.healthRegistry,
		grpc.WithTransportCredentials(a.clientCreds),
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
