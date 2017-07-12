package proxy_test

import (
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"testing"

	"code.cloudfoundry.org/loggregator/plumbing"
	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestProxy(t *testing.T) {
	grpclog.SetLogger(log.New(GinkgoWriter, "", log.LstdFlags))
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Proxy Suite")
}

type AuthorizerResult struct {
	Status       int
	ErrorMessage string
}

type LogAuthorizer struct {
	TokenParam string
	Target     string
	Result     AuthorizerResult
}

var _ = BeforeSuite(func() {
	proxy.MetricsInterval = 100 * time.Millisecond
})

func (a *LogAuthorizer) Authorize(authToken string, target string) (int, error) {
	a.TokenParam = authToken
	a.Target = target

	return a.Result.Status, errors.New(a.Result.ErrorMessage)
}

type AdminAuthorizer struct {
	TokenParam string
	Result     AuthorizerResult
}

func (a *AdminAuthorizer) Authorize(authToken string) (bool, error) {
	a.TokenParam = authToken

	return a.Result.Status == http.StatusOK, errors.New(a.Result.ErrorMessage)
}

func startListener(addr string) net.Listener {
	var lis net.Listener
	f := func() error {
		var err error
		lis, err = net.Listen("tcp", addr)
		return err
	}
	Eventually(f).ShouldNot(HaveOccurred())

	return lis
}

func startGRPCServer(ds plumbing.DopplerServer, addr string) (net.Listener, *grpc.Server) {
	lis := startListener(addr)
	s := grpc.NewServer()
	plumbing.RegisterDopplerServer(s, ds)
	go s.Serve(lis)

	return lis, s
}

type recentLogsRequest struct {
	ctx   context.Context
	appID string
}

type subscribeRequest struct {
	ctx     context.Context
	request *plumbing.SubscriptionRequest
}

type SpyGRPCConnector struct {
	mu               sync.Mutex
	subscriptions    *subscribeRequest
	subscriptionsErr error
	recentLogs       *recentLogsRequest
}

func newSpyGRPCConnector(err error) *SpyGRPCConnector {
	return &SpyGRPCConnector{
		subscriptionsErr: err,
	}
}

func (s *SpyGRPCConnector) Subscribe(ctx context.Context, req *plumbing.SubscriptionRequest) (func() ([]byte, error), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptions = &subscribeRequest{
		ctx:     ctx,
		request: req,
	}

	return func() ([]byte, error) { return []byte("a-slice"), s.subscriptionsErr }, nil
}

func (s *SpyGRPCConnector) ContainerMetrics(ctx context.Context, appID string) [][]byte {
	return nil
}
func (s *SpyGRPCConnector) RecentLogs(ctx context.Context, appID string) [][]byte {
	s.recentLogs = &recentLogsRequest{
		ctx:   ctx,
		appID: appID,
	}

	return [][]byte{
		[]byte("log1"),
		[]byte("log2"),
		[]byte("log3"),
	}
}

type valueUnit struct {
	Value float64
	Unit  string
}

type counter struct {
	total int
	tags  map[string]string
}

type mockMetricSender struct {
	mu           sync.Mutex
	valueMetrics map[string]valueUnit
	counters     map[string]counter
}

func newMockMetricSender() *mockMetricSender {
	return &mockMetricSender{
		valueMetrics: make(map[string]valueUnit),
		counters:     make(map[string]counter),
	}
}

func (m *mockMetricSender) SendValue(name string, value float64, unit string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.valueMetrics[name] = valueUnit{Value: value, Unit: unit}

	return nil
}

func (m *mockMetricSender) getValue(name string) valueUnit {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, ok := m.valueMetrics[name]
	if !ok {
		return valueUnit{Value: 0.0, Unit: ""}
	}

	return v
}

func (m *mockMetricSender) SendCounterIncrement(name string, tags map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.counters[name]
	if !ok {
		c = counter{
			total: 1,
			tags:  tags,
		}
	} else {
		c.total++
	}

	m.counters[name] = c
	return nil
}

func (m *mockMetricSender) IncrementEgressFirehose() {
	m.SendCounterIncrement("egress", map[string]string{
		"endpoint": "firehose",
	})
}

func (m *mockMetricSender) IncrementEgressStream() {
	m.SendCounterIncrement("egress", map[string]string{
		"endpoint": "stream",
	})
}

func (m *mockMetricSender) getCounter(name string) (int, map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.counters[name]
	return c.total, c.tags
}
