package app

import (
	"log"
	"time"

	"code.cloudfoundry.org/tlsconfig"

	gendiodes "code.cloudfoundry.org/go-diodes"
	"code.cloudfoundry.org/loggregator-release/src/diodes"
	"code.cloudfoundry.org/loggregator-release/src/metricemitter"
	"code.cloudfoundry.org/loggregator-release/src/plumbing"
	"code.cloudfoundry.org/loggregator-release/src/router/internal/server"
	v1 "code.cloudfoundry.org/loggregator-release/src/router/internal/server/v1"
	v2 "code.cloudfoundry.org/loggregator-release/src/router/internal/server/v2"
	"code.cloudfoundry.org/loggregator-release/src/router/internal/sinks"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Router routes envelopes from producers to any subscribers.
type Router struct {
	c      *Config
	server *server.Server
	addrs  Addrs
}

// NewRouter creates a new Router with the given options. Each provided
// RouterOption will manipulate the Router behavior.
func NewRouter(grpc GRPC, opts ...RouterOption) *Router {
	r := &Router{
		c: &Config{
			GRPC: grpc,
			Agent: Agent{
				GRPCAddress: "127.0.0.1:3458",
			},
			MetricBatchIntervalMilliseconds: 5000,
		},
	}

	for _, o := range opts {
		o(r)
	}

	return r
}

// RouterOption is used to configure a new Router.
type RouterOption func(*Router)

// WithMetricReporting returns a RouterOption that enables Router to emit
// metrics about itself.
func WithMetricReporting(
	agent Agent,
	metricBatchIntervalMilliseconds uint,
	sourceID string,
) RouterOption {
	return func(r *Router) {
		r.c.Agent = agent
		r.c.MetricBatchIntervalMilliseconds = metricBatchIntervalMilliseconds
		r.c.MetricSourceID = sourceID
	}
}

// WithBufferSizes returns a routerOption that enables configurable
// buffer sizes
func WithBufferSizes(
	ingressBufferSize int,
	egressBufferSize int,
) RouterOption {
	return func(r *Router) {
		r.c.IngressBufferSize = ingressBufferSize
		r.c.EgressBufferSize = egressBufferSize
	}
}

// Start enables the Router to start receiving envelope, accepting
// subscriptions and routing data.
func (d *Router) Start() {
	log.Printf("Startup: Setting up the router server")

	//------------------------------
	// v2 Metrics (gRPC)
	//------------------------------
	metricClient := initV2Metrics(d.c)

	//------------------------------
	// Ingress (gRPC v1 and v2)
	// Egress  (gRPC v1 and v2)
	//------------------------------

	// metric-documentation-v2: (loggregator.doppler.dropped) Number of
	// envelopes dropped by the diode inbound from metron
	ingressDropped := metricClient.NewCounter("dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{"direction": "ingress"}),
	)

	// metric-documentation-v2: (loggregator.doppler.dropped) Number of
	// envelopes dropped by the outbound diode to subscribers
	droppedEgress := metricClient.NewCounter("dropped",
		metricemitter.WithVersion(2, 0),
		metricemitter.WithTags(map[string]string{"direction": "egress"}),
	)

	// metric-documentation-v2: (loggregator.doppler.ingress) Number of received
	// envelopes from Metron on Doppler's v2 gRPC server
	ingress := metricClient.NewCounter("ingress",
		metricemitter.WithVersion(2, 0),
	)

	v1Buf := diodes.NewManyToOneEnvelope(d.c.IngressBufferSize, gendiodes.AlertFunc(func(missed int) {
		log.Printf("Dropped %d envelopes (v1 buffer)", missed)

		ingressDropped.Increment(uint64(missed)) // nolint:gosec
	}))

	v2Buf := diodes.NewManyToOneEnvelopeV2(d.c.IngressBufferSize, gendiodes.AlertFunc(func(missed int) {
		log.Printf("Dropped %d envelopes (v2 buffer)", missed)

		ingressDropped.Increment(uint64(missed)) // nolint:gosec
	}))

	// metric-documentation-v2: (loggregator.doppler.subscriptions) Number of
	// active subscriptions for both V1 and V2 egress APIs.
	subscriptionsMetric := metricClient.NewGauge("subscriptions", "subscriptions",
		metricemitter.WithVersion(2, 0),
	)

	v1Ingress := v1.NewIngestorServer(
		v1Buf,
		v2Buf,
		ingress,
	)
	v1Router := v1.NewRouter()
	v1Egress := v1.NewDopplerServer(
		v1Router,
		metricClient,
		droppedEgress,
		subscriptionsMetric,
		100*time.Millisecond,
		100,
		d.c.EgressBufferSize,
	)
	v2Ingress := v2.NewIngressServer(
		v1Buf,
		v2Buf,
		ingress,
	)
	v2PubSub := v2.NewPubSub()
	v2Egress := v2.NewEgressServer(
		v2PubSub,
		metricClient,
		droppedEgress,
		subscriptionsMetric,
		100*time.Millisecond,
		100,
	)

	tlsConf, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(d.c.GRPC.CertFile, d.c.GRPC.KeyFile),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(d.c.GRPC.CAFile),
	)

	if len(d.c.GRPC.CipherSuites) > 0 {
		opt := plumbing.WithCipherSuites(d.c.GRPC.CipherSuites)
		opt(tlsConf)
	}

	if err != nil {
		log.Panicf("Failed to create tls config for router server: %s", err)
	}

	srv, err := server.NewServer(
		d.c.GRPC.Port,
		v1Ingress,
		v1Egress,
		v2Ingress,
		v2Egress,
		grpc.Creds(credentials.NewTLS(tlsConf)),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.MaxRecvMsgSize(10*1024*1024),
	)
	if err != nil {
		log.Panicf("Failed to create router server: %s", err)
	}

	d.server = srv
	d.addrs.GRPC = d.server.Addr()

	//------------------------------
	// Start
	//------------------------------
	messageRouter := sinks.NewMessageRouter(v1Router)
	go messageRouter.Start(v1Buf)

	repeater := v2.NewRepeater(v2PubSub.Publish, v2Buf.Next)
	go repeater.Start()

	go d.server.Start()

	log.Print("Startup: router server started.")
}

// Addrs stores listener addresses of the router process.
type Addrs struct {
	GRPC string
}

// Addrs returns a copy of the listeners' addresses.
func (d *Router) Addrs() Addrs {
	return d.addrs
}

// Stop closes the gRPC listeners.
func (d *Router) Stop() {
	// TODO: Drain
	d.server.Stop()
}

func initV2Metrics(c *Config) *metricemitter.Client {
	credentials, err := plumbing.NewClientCredentials(
		c.GRPC.CertFile,
		c.GRPC.KeyFile,
		c.GRPC.CAFile,
		"metron",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	batchInterval := time.Duration(c.MetricBatchIntervalMilliseconds) * time.Millisecond //#nosec G115

	// metric-documentation-v2: setup function
	metricClient, err := metricemitter.NewClient(
		c.Agent.GRPCAddress,
		metricemitter.WithGRPCDialOptions(grpc.WithTransportCredentials(credentials)),
		metricemitter.WithOrigin("loggregator.doppler"),
		metricemitter.WithPulseInterval(batchInterval),
		metricemitter.WithSourceID(c.MetricSourceID),
	)
	if err != nil {
		log.Fatalf("Could not configure metric emitter: %s", err)
	}

	return metricClient
}
