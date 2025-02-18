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
	"github.com/prometheus/client_golang/prometheus/collectors"
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

// counterVec allows us to hide the prometheus logic from the user. [prometheus.CounterVec] is not
// an actual metric but more of a metric factory which returns a metric for each set of labels.
// To not leak the returned prometheus types, this type is used.
type counterVec struct {
	vec *prometheus.CounterVec
}

func (c counterVec) Add(f float64, labels []string) {
	c.vec.WithLabelValues(labels...).Add(f)
}

type CounterVec interface {
	// Add to metric, the number of labels must match the number of label names that were
	// given when the [CounterVec] was created.
	Add(float64, []string)
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

// histogramVec allows us to hide the prometheus logic from the user
type histogramVec struct {
	vec *prometheus.HistogramVec
}

func (c histogramVec) Observe(f float64, labels []string) {
	c.vec.WithLabelValues(labels...).Observe(f)
}

type HistogramVec interface {
	// Observe the metric, the number of labels must match the number of label names that were
	// given when the [HistogramVec] was created.
	Observe(f float64, labels []string)
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

	pr.mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Registry: pr.registerer,
	}))
	return pr
}

func (p *Registry) RegisterDebugMetrics() {
	p.registerer.MustRegister(collectors.NewGoCollector())
	p.registerer.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

// Creates new counter. When a duplicate is registered, the Registry will return
// the previously created metric.
func (p *Registry) NewCounter(name, helpText string, opts ...MetricOption) Counter {
	opt := toPromOpt(name, helpText, opts...)
	c := prometheus.NewCounter(prometheus.CounterOpts(opt))
	return p.registerCollector(name, c).(Counter)
}

// Creates new counter vector. When a duplicate is registered, the Registry will return
// the previously created metric.
func (p *Registry) NewCounterVec(name, helpText string, labelNames []string, opts ...MetricOption) CounterVec {
	opt := toPromOpt(name, helpText, opts...)
	c := prometheus.NewCounterVec(prometheus.CounterOpts(opt), labelNames)
	// See [counterVec] for details.
	return counterVec{vec: p.registerCollector(name, c).(*prometheus.CounterVec)}
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

// NewHistogramVec creates new histogram vector. When a duplicate is registered, the Registry will return
// the previously created metric.
func (p *Registry) NewHistogramVec(name, helpText string, labelNames []string, buckets []float64, opts ...MetricOption) HistogramVec {
	c := prometheus.NewHistogramVec(toHistogramOpts(name, helpText, buckets, opts...), labelNames)
	// See [histogramVec] for details.
	return histogramVec{vec: p.registerCollector(name, c).(*prometheus.HistogramVec)}
}

func (p *Registry) RemoveGauge(g Gauge) {
	p.registerer.Unregister(g.(prometheus.Collector))
}

func (p *Registry) RemoveHistogram(h Histogram) {
	p.registerer.Unregister(h.(prometheus.Collector))
}

func (p *Registry) RemoveHistogramVec(hv HistogramVec) {
	if hvIntern, ok := hv.(histogramVec); ok {
		p.registerer.Unregister(hvIntern.vec)
	}
}

func (p *Registry) RemoveCounter(c Counter) {
	p.registerer.Unregister(c.(prometheus.Collector))
}

func (p *Registry) RemoveCounterVec(cv CounterVec) {
	if cvIntern, ok := cv.(counterVec); ok {
		p.registerer.Unregister(cvIntern.vec)
	}
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
		Addr:              addr,
		Handler:           p.mux,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		p.loggr.Fatalf("Unable to setup metrics endpoint (%s): %s", addr, err)
	}
	p.loggr.Printf("Metrics endpoint is listening on %s/metrics", lis.Addr().String())

	parts := strings.Split(lis.Addr().String(), ":")
	p.port = parts[len(parts)-1]

	go s.Serve(lis) //nolint:errcheck
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
		Addr:              addr,
		Handler:           p.mux,
		TLSConfig:         tlsConfig,
		ReadTimeout:       5 * time.Minute,
		ReadHeaderTimeout: 5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
	}

	lis, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		p.loggr.Fatalf("Unable to setup metrics endpoint (%s): %s", addr, err)
	}
	p.loggr.Printf("Metrics endpoint is listening on %s", lis.Addr().String())

	parts := strings.Split(lis.Addr().String(), ":")
	p.port = parts[len(parts)-1]

	go s.Serve(lis) //nolint:errcheck
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
