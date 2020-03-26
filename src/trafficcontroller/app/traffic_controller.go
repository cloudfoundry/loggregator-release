package app

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"code.cloudfoundry.org/tlsconfig"

	logcache "code.cloudfoundry.org/log-cache/pkg/client"
	"code.cloudfoundry.org/loggregator/metricemitter"
	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/profiler"
	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/auth"
	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"

	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/keepalive"
)

// MetricClient can be used to emit metrics and events.
type MetricClient interface {
	NewCounter(name string, opts ...metricemitter.MetricOption) *metricemitter.Counter
	NewGauge(name string, unit string, opts ...metricemitter.MetricOption) *metricemitter.Gauge
	EmitEvent(title string, body string)
}

type TrafficController struct {
	conf                 *Config
	disableAccessControl bool
	metricClient         MetricClient
	uaaHTTPClient        *http.Client
	ccHTTPClient         *http.Client
}

func NewTrafficController(
	c *Config,
	disableAccessControl bool,
	metricClient MetricClient,
	uaaHTTPClient *http.Client,
	ccHTTPClient *http.Client,
) *TrafficController {
	return &TrafficController{
		conf:                 c,
		disableAccessControl: disableAccessControl,
		metricClient:         metricClient,
		uaaHTTPClient:        uaaHTTPClient,
		ccHTTPClient:         ccHTTPClient,
	}
}

func (t *TrafficController) Start() {
	log.Print("Startup: Setting up the loggregator traffic controller")

	logAuthorizer := auth.NewLogAccessAuthorizer(
		t.ccHTTPClient,
		t.disableAccessControl,
		t.conf.ApiHost,
	)

	uaaClient := auth.NewUaaClient(
		t.uaaHTTPClient,
		t.conf.UaaHost,
		t.conf.UaaClient,
		t.conf.UaaClientSecret,
	)
	adminAuthorizer := auth.NewAdminAccessAuthorizer(t.disableAccessControl, &uaaClient)

	creds, err := plumbing.NewClientCredentials(
		t.conf.GRPC.CertFile,
		t.conf.GRPC.KeyFile,
		t.conf.GRPC.CAFile,
		"doppler",
	)
	if err != nil {
		log.Fatalf("Could not use GRPC creds for server: %s", err)
	}

	f := plumbing.NewStaticFinder(t.conf.RouterAddrs)
	f.Start()

	kp := keepalive.ClientParameters{
		Time:                15 * time.Second,
		Timeout:             20 * time.Second,
		PermitWithoutStream: true,
	}

	pool := plumbing.NewPool(
		20,
		grpc.WithTransportCredentials(creds),
		grpc.WithKeepaliveParams(kp),
		grpc.WithDisableServiceConfig(),
	)
	grpcConnector := plumbing.NewGRPCConnector(1000, pool, f, t.metricClient)

	var logCacheClient proxy.LogCacheClient
	recentLogsEnabled := false

	if t.conf.LogCacheAddr != "" {
		logCacheCreds, err := plumbing.NewClientCredentials(
			t.conf.LogCacheTLSConfig.CertFile,
			t.conf.LogCacheTLSConfig.KeyFile,
			t.conf.LogCacheTLSConfig.CAFile,
			t.conf.LogCacheTLSConfig.ServerName,
		)
		if err != nil {
			log.Fatalf("Could not use LogCache creds for server: %s", err)
		}

		logCacheClient = logcache.NewClient(
			t.conf.LogCacheAddr,
			logcache.WithViaGRPC(
				grpc.WithTransportCredentials(logCacheCreds),
				grpc.WithBalancerName(roundrobin.Name),
				grpc.WithBlock(),
			),
		)
		recentLogsEnabled = true
	}

	recentLogsHandler := proxy.NewRecentLogsHandler(logCacheClient, 5*time.Second, t.metricClient, recentLogsEnabled)

	dopplerHandler := http.Handler(
		proxy.NewDopplerProxy(
			logAuthorizer,
			adminAuthorizer,
			grpcConnector,
			"doppler."+t.conf.SystemDomain,
			5*time.Second,
			t.metricClient,
			recentLogsHandler,
			t.disableAccessControl,
		),
	)

	var accessMiddleware func(http.Handler) *auth.AccessHandler
	if t.conf.SecurityEventLog != "" {
		accessLog, err := os.OpenFile(t.conf.SecurityEventLog, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			log.Panicf("Unable to open access log: %s", err)
		}
		defer func() {
			accessLog.Sync()
			accessLog.Close()
		}()
		accessLogger := auth.NewAccessLogger(accessLog)
		accessMiddleware = auth.Access(accessLogger, t.conf.IP, t.conf.OutgoingDropsondePort)
	}

	if accessMiddleware != nil {
		dopplerHandler = accessMiddleware(dopplerHandler)
	}

	go t.startServer(dopplerHandler)
	go profiler.New(t.conf.PProfPort).Start()

	killChan := make(chan os.Signal)
	signal.Notify(killChan, os.Interrupt)
	<-killChan
	log.Print("Shutting down")
}

func (t *TrafficController) startServer(dopplerHandler http.Handler) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", t.conf.OutgoingDropsondePort))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("ws bound to: %s", lis.Addr())

	server := http.Server{
		Handler:   dopplerHandler,
		TLSConfig: t.buildTLSConfig(),
	}

	log.Fatal(server.ServeTLS(lis, "", ""))
}

func (t *TrafficController) buildTLSConfig() *tls.Config {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(t.conf.OutgoingCertFile, t.conf.OutgoingKeyFile),
	).Server()

	if err != nil {
		log.Fatal(err)
	}
	return tlsConfig
}
