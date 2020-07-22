package metrics

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/tlsconfig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// The Registry keeps track of registered counters and gauges. Optionally, it can
// provide a server on a Prometheus-formatted endpoint.
type Registry struct {
	port       string
	loggr      *log.Logger
	registerer prometheus.Registerer
	mux        *http.ServeMux
}

// A cumulative metric that represents a single monotonically increasing counter
// whose value can only increase or be reset to zero on restart
type Counter interface {
	Add(float64)
}

// A single numerical value that can arbitrarily go up and down.
type Gauge interface {
	Add(float64)
	Set(float64)
}

// A histogram counts observations into buckets.
type Histogram interface {
	Observe(float64)
}

// Registry will register the metrics route with the default http mux but will not
// start an http server. This is intentional so that we can combine metrics with
// other things like pprof into one server. To start a server
// just for metrics, use the WithServer RegistryOption
func NewRegistry(logger *log.Logger, opts ...RegistryOption) *Registry {
	pr := &Registry{
		loggr: logger,
		mux:   http.NewServeMux(),
	}

	for _, o := range opts {
		o(pr)
	}

	registry := prometheus.NewRegistry()
	pr.registerer = registry

	pr.registerer.MustRegister(prometheus.NewGoCollector())
	pr.registerer.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	pr.mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Registry: pr.registerer,
	}))
	return pr
}

// Creates new counter. When a duplicate is registered, the Registry will return
// the previously created metric.
func (p *Registry) NewCounter(name, helpText string, opts ...MetricOption) Counter {
	opt := toPromOpt(name, helpText, opts...)
	c := prometheus.NewCounter(prometheus.CounterOpts(opt))
	return p.registerCollector(name, c).(Counter)
}

// Creates new gauge. When a duplicate is registered, the Registry will return
// the previously created metric.
func (p *Registry) NewGauge(name, helpText string, opts ...MetricOption) Gauge {
	opt := toPromOpt(name, helpText, opts...)
	g := prometheus.NewGauge(prometheus.GaugeOpts(opt))
	return p.registerCollector(name, g).(Gauge)
}

// Creates new histogram. When a duplicate is registered, the Registry will return
// the previously created metric.
func (p *Registry) NewHistogram(name, helpText string, buckets []float64, opts ...MetricOption) Histogram {
	h := prometheus.NewHistogram(toHistogramOpts(name, helpText, buckets, opts...))
	return p.registerCollector(name, h).(Histogram)
}

func (p *Registry) registerCollector(name string, c prometheus.Collector) prometheus.Collector {
	err := p.registerer.Register(c)
	if err != nil {
		typ, ok := err.(prometheus.AlreadyRegisteredError)
		if !ok {
			p.loggr.Panicf("unable to create %s: %s", name, err)
		}

		return typ.ExistingCollector
	}

	return c
}

// Get the port of the running metrics server
func (p *Registry) Port() string {
	return fmt.Sprint(p.port)
}

func toPromOpt(name, helpText string, mOpts ...MetricOption) prometheus.Opts {
	opt := prometheus.Opts{
		Name:        name,
		Help:        helpText,
		ConstLabels: make(map[string]string),
	}

	for _, o := range mOpts {
		o(&opt)
	}

	return opt
}

func toHistogramOpts(name, helpText string, buckets []float64, mOpts ...MetricOption) prometheus.HistogramOpts {
	promOpt := toPromOpt(name, helpText, mOpts...)

	return prometheus.HistogramOpts{
		Namespace:   promOpt.Namespace,
		Subsystem:   promOpt.Subsystem,
		Name:        promOpt.Name,
		Help:        promOpt.Help,
		ConstLabels: promOpt.ConstLabels,
		Buckets:     buckets,
	}
}

// Options for registry initialization
type RegistryOption func(r *Registry)

// Starts an http server on localhost at the given port to host metrics.
func WithServer(port int) RegistryOption {
	return func(r *Registry) {
		r.start("127.0.0.1", port)
	}
}

// Starts an https server on localhost at the given port to host metrics.
func WithTLSServer(port int, certFile, keyFile, caFile string) RegistryOption {
	return func(r *Registry) {
		r.startTLS(port, certFile, keyFile, caFile)
	}
}

// Starts an http server on the given port to host metrics.
func WithPublicServer(port int) RegistryOption {
	return func(r *Registry) {
		r.start("0.0.0.0", port)
	}
}

func (p *Registry) start(ipAddr string, port int) {
	addr := fmt.Sprintf("%s:%d", ipAddr, port)
	s := http.Server{
		Addr:         addr,
		Handler:      p.mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		p.loggr.Fatalf("Unable to setup metrics endpoint (%s): %s", addr, err)
	}
	p.loggr.Printf("Metrics endpoint is listening on %s/metrics", lis.Addr().String())

	parts := strings.Split(lis.Addr().String(), ":")
	p.port = parts[len(parts)-1]

	go s.Serve(lis)
}

func (p *Registry) startTLS(port int, certFile, keyFile, caFile string) {
	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certFile, keyFile),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(caFile),
	)
	if err != nil {
		p.loggr.Fatalf("unable to generate server TLS Config: %s", err)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	s := http.Server{
		Addr:         addr,
		Handler:      p.mux,
		TLSConfig:    tlsConfig,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	lis, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		p.loggr.Fatalf("Unable to setup metrics endpoint (%s): %s", addr, err)
	}
	p.loggr.Printf("Metrics endpoint is listening on %s", lis.Addr().String())

	parts := strings.Split(lis.Addr().String(), ":")
	p.port = parts[len(parts)-1]

	go s.Serve(lis)
}

// Options applied to metrics on creation
type MetricOption func(o *prometheus.Opts)

// Add these tags to the metrics
func WithMetricLabels(labels map[string]string) MetricOption {
	return func(o *prometheus.Opts) {
		for k, v := range labels {
			o.ConstLabels[k] = v
		}
	}
}
