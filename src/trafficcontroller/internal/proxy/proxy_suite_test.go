package proxy_test

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"testing"

	"code.cloudfoundry.org/loggregator-release/src/plumbing"
	"code.cloudfoundry.org/loggregator-release/src/trafficcontroller/internal/proxy"

	"google.golang.org/grpc/grpclog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProxy(t *testing.T) {
	l := grpclog.NewLoggerV2(GinkgoWriter, GinkgoWriter, GinkgoWriter)
	grpclog.SetLoggerV2(l)

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

type subscribeRequest struct {
	ctx     context.Context
	request *plumbing.SubscriptionRequest
}

type SpyGRPCConnector struct {
	mu               sync.Mutex
	subscriptions    *subscribeRequest
	subscriptionsErr error
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
