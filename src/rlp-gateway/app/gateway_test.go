package app_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"expvar"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/plumbing" // TODO: Resolve duplicate proto error
	"code.cloudfoundry.org/loggregator/rlp-gateway/app"
	"code.cloudfoundry.org/loggregator/rlp-gateway/internal/metrics"
	"code.cloudfoundry.org/loggregator/testservers"
	"google.golang.org/grpc"

	"github.com/gogo/protobuf/jsonpb"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gateway", func() {
	var (
		logsProvider *stubLogsProvider
		cfg          app.Config

		logger  = log.New(GinkgoWriter, "", log.LstdFlags)
		metrics = metrics.New(expvar.NewMap("RLPGateway"))
	)

	BeforeEach(func() {
		cfg = app.Config{
			LogsProviderCAPath:         testservers.Cert("loggregator-ca.crt"),
			LogsProviderClientCertPath: testservers.Cert("rlpgateway.crt"),
			LogsProviderClientKeyPath:  testservers.Cert("rlpgateway.key"),
			LogsProviderCommonName:     "reverselogproxy",

			StreamTimeout: 10 * time.Minute,

			HTTP: app.HTTP{
				GatewayAddr: ":0",
				CertPath:    testservers.Cert("localhost.crt"),
				KeyPath:     testservers.Cert("localhost.key"),
			},

			LogAccessAuthorization: app.LogAccessAuthorization{
				CertPath:   testservers.Cert("capi.crt"),
				KeyPath:    testservers.Cert("capi.key"),
				CAPath:     testservers.Cert("loggregator-ca.crt"),
				CommonName: "capi",
			},

			LogAdminAuthorization: app.LogAdminAuthorization{
				ClientID:     "client",
				ClientSecret: "secret",
				CAPath:       testservers.Cert("loggregator-ca.crt"),
			},
		}
	})

	Context("with valid log access authorization", func() {
		BeforeEach(func() {
			internalLogAccess := newAuthorizationServer(
				http.StatusOK,
				"{}",
				testservers.Cert("capi.crt"),
				testservers.Cert("capi.key"),
			)
			logAdmin := newAuthorizationServer(
				http.StatusBadRequest,
				invalidTokenResponse,
				testservers.Cert("uaa.crt"),
				testservers.Cert("uaa.key"),
			)

			logsProvider = newStubLogsProvider()
			logsProvider.toSend = 10

			cfg.LogsProviderAddr = logsProvider.addr()
			cfg.LogAccessAuthorization.Addr = internalLogAccess.URL
			cfg.LogAdminAuthorization.Addr = logAdmin.URL
		})

		It("forwards requests to RLP", func() {
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient()
			go client.open("https://" + gateway.Addr() + "/v2/read?log&source_id=deadbeef-dead-dead-dead-deaddeafbeef")

			Eventually(client.envelopes).Should(HaveLen(10))
		})

		It("doesn't panic when the logs provider closes", func() {
			logsProvider.toSend = 1

			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient()
			Expect(func() {
				client.open("https://" + gateway.Addr() + "/v2/read?log&source_id=deadbeef-dead-dead-dead-deaddeafbeef")
			}).ToNot(Panic())
		})

		It("does not accept unencrypted connections", func () {
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient()
			resp, err := client.open("http://" + gateway.Addr() + "/v2/read?log&source_id=deadbeef")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Context("when the user is an admin", func() {
		BeforeEach(func() {
			internalLogAccess := newAuthorizationServer(
				http.StatusBadRequest,
				"{}",
				testservers.Cert("capi.crt"),
				testservers.Cert("capi.key"),
			)
			logAdmin := newAuthorizationServer(
				http.StatusOK,
				validTokenResponse,
				testservers.Cert("uaa.crt"),
				testservers.Cert("uaa.key"),
			)

			logsProvider = newStubLogsProvider()
			logsProvider.toSend = 10

			cfg.LogsProviderAddr = logsProvider.addr()
			cfg.LogAccessAuthorization.Addr = internalLogAccess.URL
			cfg.LogAdminAuthorization.Addr = logAdmin.URL
		})

		It("forwards HTTP requests to RLP", func() {
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient()
			go client.open("https://" + gateway.Addr() + "/v2/read?log")

			Eventually(client.envelopes).Should(HaveLen(10))
		})

		It("doesn't panic when the logs provider closes", func() {
			logsProvider.toSend = 1

			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient()
			Expect(func() {
				client.open("https://" + gateway.Addr() + "/v2/read?log")
			}).ToNot(Panic())
		})
	})

	Context("when the user does not have log access", func() {
		BeforeEach(func() {
			internalLogAccess := newAuthorizationServer(
				http.StatusBadRequest,
				"{}",
				testservers.Cert("capi.crt"),
				testservers.Cert("capi.key"),
			)
			logAdmin := newAuthorizationServer(
				http.StatusBadRequest,
				invalidTokenResponse,
				testservers.Cert("uaa.crt"),
				testservers.Cert("uaa.key"),
			)

			logsProvider = newStubLogsProvider()

			cfg.LogsProviderAddr = logsProvider.addr()
			cfg.LogAccessAuthorization.Addr = internalLogAccess.URL
			cfg.LogAdminAuthorization.Addr = logAdmin.URL
		})

		It("returns 404 if user is not authorized for log access for a source ID", func() {
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient()
			resp, err := client.open("https://" + gateway.Addr() + "/v2/read?log&source_id=some-id")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("returns 404 if user is not authorized as admin", func() {
			gateway := app.NewGateway(cfg, metrics, logger, GinkgoWriter)
			gateway.Start(false)
			defer gateway.Stop()

			client := newTestClient()
			resp, err := client.open("https://" + gateway.Addr() + "/v2/read?log")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})
})

var (
	invalidTokenResponse = `{
		"invalidToken": "invalid_token",
		"error_description": "Invalid token (could not decode): invalidToken"
	}`
	validTokenResponse = `{
		"scope": ["logs.admin"]
	}`
)

type stubLogsProvider struct {
	listener net.Listener
	toSend   int
}

func newStubLogsProvider() *stubLogsProvider {
	creds, err := plumbing.NewServerCredentials(
		testservers.Cert("reverselogproxy.crt"),
		testservers.Cert("reverselogproxy.key"),
		testservers.Cert("loggregator-ca.crt"),
	)
	if err != nil {
		panic(err)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	s := &stubLogsProvider{
		listener: l,
	}

	server := grpc.NewServer(grpc.Creds(creds))
	loggregator_v2.RegisterEgressServer(server, s)

	go func() {
		err := server.Serve(s.listener)
		fmt.Println(err)
	}()

	return s
}

func (s *stubLogsProvider) Receiver(*loggregator_v2.EgressRequest, loggregator_v2.Egress_ReceiverServer) error {
	panic("not implemented")
}

func (s *stubLogsProvider) BatchedReceiver(req *loggregator_v2.EgressBatchRequest, srv loggregator_v2.Egress_BatchedReceiverServer) error {
	for i := 0; i < s.toSend; i++ {
		if isDone(srv.Context()) {
			break
		}

		srv.Send(&loggregator_v2.EnvelopeBatch{
			Batch: []*loggregator_v2.Envelope{
				{Timestamp: time.Now().UnixNano()},
			},
		})
	}

	return nil
}

func (s *stubLogsProvider) addr() string {
	return s.listener.Addr().String()
}

type testClient struct {
	mu         sync.Mutex
	_envelopes []*loggregator_v2.Envelope
}

func newTestClient() *testClient {
	return &testClient{}
}

func (tc *testClient) open(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "my-token")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	if resp.StatusCode != 200 {
		return resp, nil
	}

	buf := bytes.NewBuffer(nil)
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		switch {
		case bytes.HasPrefix(line, []byte("data:")):
			buf.Write(line[6:])
		case bytes.Equal(line, []byte("\n")):
			var batch loggregator_v2.EnvelopeBatch
			if err := jsonpb.Unmarshal(buf, &batch); err != nil {
				panic(fmt.Sprintf("failed to unmarshal envelopes: %s", err))
			}
			tc.mu.Lock()
			tc._envelopes = append(tc._envelopes, batch.GetBatch()...)
			tc.mu.Unlock()
		default:
			panic("unhandled SSE data")
		}
	}
}

func (tc *testClient) envelopes() []*loggregator_v2.Envelope {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	return tc._envelopes
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

type authorizationServer struct {
	statusCode   int
	responseBody string
}

func newAuthorizationServer(code int, body, crtFile, keyFile string) *httptest.Server {
	as := &authorizationServer{
		statusCode:   code,
		responseBody: body,
	}

	return as.startTLS(crtFile, keyFile)
}

func (as *authorizationServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(as.statusCode)
	fmt.Fprintf(w, "%s", as.responseBody)
}

func (as *authorizationServer) startTLS(crt, key string) *httptest.Server {
	s := httptest.NewUnstartedServer(as)
	cert, err := tls.LoadX509KeyPair(crt, key)
	if err != nil {
		panic(fmt.Sprintf("httptest: NewTLSServer: %v", err))
	}

	s.TLS = new(tls.Config)
	s.TLS.NextProtos = []string{"http/1.1"}
	s.TLS.Certificates = []tls.Certificate{cert}
	s.Start()

	return s
}
