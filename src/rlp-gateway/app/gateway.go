package app

import (
	"crypto/x509"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/rlp-gateway/internal/auth"
	"code.cloudfoundry.org/loggregator/rlp-gateway/internal/ingress"
	"code.cloudfoundry.org/loggregator/rlp-gateway/internal/metrics"
	"code.cloudfoundry.org/loggregator/rlp-gateway/internal/web"
	"github.com/gorilla/handlers"
)

// Gateway provides a high level for running the RLP gateway
type Gateway struct {
	cfg           Config
	listener      net.Listener
	server        *http.Server
	log           *log.Logger
	metrics       *metrics.Metrics
	httpLogOutput io.Writer
}

// NewGateway creates a new Gateway
func NewGateway(
	cfg Config,
	metrics *metrics.Metrics,
	log *log.Logger,
	httpLogOutput io.Writer,
) *Gateway {
	return &Gateway{
		cfg:           cfg,
		log:           log,
		metrics:       metrics,
		httpLogOutput: httpLogOutput,
	}
}

// Start will start the process that connects to the logs provider
// and listens on http
func (g *Gateway) Start(blocking bool) {
	creds, err := plumbing.NewClientCredentials(
		g.cfg.LogsProviderClientCertPath,
		g.cfg.LogsProviderClientKeyPath,
		g.cfg.LogsProviderCAPath,
		g.cfg.LogsProviderCommonName,
	)
	if err != nil {
		g.log.Fatalf("failed to load client TLS config: %s", err)
	}

	uaaClient := auth.NewUAAClient(
		g.cfg.LogAdminAuthorization.Addr,
		g.cfg.LogAdminAuthorization.ClientID,
		g.cfg.LogAdminAuthorization.ClientSecret,
		buildAdminAuthClient(g.cfg),
		g.metrics,
		g.log,
	)

	capiClient := auth.NewCAPIClient(
		g.cfg.LogAccessAuthorization.Addr,
		g.cfg.LogAccessAuthorization.ExternalAddr,
		buildAccessAuthorizationClient(g.cfg),
		g.metrics,
		g.log,
	)

	middlewareProvider := web.NewCFAuthMiddlewareProvider(
		uaaClient,
		capiClient,
	)

	lc := ingress.NewLogClient(creds, g.cfg.LogsProviderAddr)
	stack := handlers.RecoveryHandler(handlers.PrintRecoveryStack(true))(
		handlers.LoggingHandler(
			g.httpLogOutput,
			middlewareProvider.Middleware(web.NewHandler(
				lc,
				g.cfg.StreamTimeout,
			)),
		),
	)

	l, err := net.Listen("tcp", g.cfg.HTTP.GatewayAddr)
	if err != nil {
		g.log.Fatalf("failed to start listener: %s", err)
	}
	g.log.Printf("http bound to: %s", l.Addr().String())

	g.listener = l
	g.server = &http.Server{
		Addr:    g.cfg.HTTP.GatewayAddr,
		Handler: stack,
	}

	if blocking {
		g.server.ServeTLS(g.listener, g.cfg.HTTP.CertPath, g.cfg.HTTP.KeyPath)
		return
	}

	go g.server.ServeTLS(g.listener, g.cfg.HTTP.CertPath, g.cfg.HTTP.KeyPath)
}

// Stop closes the server connection
func (g *Gateway) Stop() {
	_ = g.server.Close()
}

// Addr returns the address the gateway HTTP listener is bound to
func (g *Gateway) Addr() string {
	if g.listener == nil {
		return ""
	}

	return g.listener.Addr().String()
}

func buildAdminAuthClient(cfg Config) *http.Client {
	tlsConfig := plumbing.NewTLSConfig()
	tlsConfig.InsecureSkipVerify = cfg.SkipCertVerify
	tlsConfig.RootCAs = loadUaaCA(cfg.LogAdminAuthorization.CAPath)

	transport := &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		DisableKeepAlives:   true,
	}

	return &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
	}
}

func buildAccessAuthorizationClient(cfg Config) *http.Client {
	tlsConfig, err := plumbing.NewClientMutualTLSConfig(
		cfg.LogAccessAuthorization.CertPath,
		cfg.LogAccessAuthorization.KeyPath,
		cfg.LogAccessAuthorization.CAPath,
		cfg.LogAccessAuthorization.CommonName,
	)
	if err != nil {
		log.Fatalf("unable to create log access HTTP Client: %s", err)
	}

	tlsConfig.InsecureSkipVerify = cfg.SkipCertVerify
	transport := &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		DisableKeepAlives:   true,
	}

	return &http.Client{
		Timeout:   20 * time.Second,
		Transport: transport,
	}
}

func loadUaaCA(uaaCertPath string) *x509.CertPool {
	caCert, err := ioutil.ReadFile(uaaCertPath)
	if err != nil {
		log.Fatalf("failed to read UAA CA certificate: %s", err)
	}

	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		log.Fatal("failed to parse UAA CA certificate.")
	}

	return certPool
}
